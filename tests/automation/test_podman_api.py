from __future__ import annotations

import ast
import importlib
import os
from collections.abc import Callable, Sequence
from contextlib import AbstractContextManager
from pathlib import Path
from types import ModuleType
from typing import Any

import pytest
from podman.errors import APIError


class FakeContainer:
    def __init__(self, health: Sequence[str] = ("healthy",)) -> None:
        self._health = iter(health)
        self._current_health = health[-1]
        self.exec_calls: list[tuple[list[str], dict[str, object]]] = []
        self.exec_result: tuple[int, tuple[bytes | None, bytes | None]] = (
            0,
            (b"", b""),
        )

    @property
    def attrs(self) -> dict[str, object]:
        self._current_health = next(self._health, self._current_health)
        return {"State": {"Health": {"Status": self._current_health}}}

    def reload(self) -> None:
        return None

    def exec_run(self, arguments: list[str], **kwargs: object) -> object:
        self.exec_calls.append((arguments, kwargs))
        return self.exec_result


class FakeContainers:
    def __init__(self, containers: Sequence[FakeContainer] = ()) -> None:
        self.result = list(containers)
        self.calls: list[dict[str, object]] = []
        self.error: APIError | None = None

    def list(self, **kwargs: object) -> Sequence[FakeContainer]:
        self.calls.append(kwargs)
        if self.error is not None:
            raise self.error
        return self.result


class FakeClient:
    def __init__(self, containers: FakeContainers) -> None:
        self.containers = containers
        self.entered = 0
        self.exited = 0

    def __enter__(self) -> FakeClient:
        self.entered += 1
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_value: BaseException | None,
        traceback: object,
    ) -> None:
        self.exited += 1


class FakeClock:
    def __init__(self) -> None:
        self.now = 0.0
        self.sleeps: list[float] = []

    def __call__(self) -> float:
        return self.now

    def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)
        self.now += seconds


@pytest.fixture
def podman_api() -> ModuleType:
    try:
        return importlib.import_module("scripts.automation.podman_api")
    except ModuleNotFoundError:
        pytest.fail("scripts.automation.podman_api is not implemented", pytrace=False)


def _connect(
    podman_api: ModuleType,
    client: FakeClient,
    *,
    env: dict[str, str] | None = None,
    exists: Callable[[Path], bool] | None = None,
    uid_factory: Callable[[], int] | None = None,
    clock: Callable[[], float] | None = None,
    sleep: Callable[[float], None] | None = None,
) -> AbstractContextManager[Any]:
    arguments: dict[str, object] = {
        "client_factory": lambda: client,
        "env": env if env is not None else {"CONTAINER_HOST": "unix:///explicit.sock"},
    }
    if exists is not None:
        arguments["exists"] = exists
    if uid_factory is not None:
        arguments["uid_factory"] = uid_factory
    if clock is not None:
        arguments["clock"] = clock
    if sleep is not None:
        arguments["sleep"] = sleep
    return podman_api.connect("project-a", "dev", **arguments)


def test_explicit_container_host_has_precedence_and_is_restored(
    podman_api: ModuleType,
) -> None:
    client = FakeClient(FakeContainers())
    env = {"CONTAINER_HOST": "unix:///explicit.sock", "KEEP": "value"}
    exists_calls: list[Path] = []

    def factory() -> FakeClient:
        assert env["CONTAINER_HOST"] == "unix:///explicit.sock"
        return client

    with podman_api.connect(
        "project-a",
        "dev",
        client_factory=factory,
        env=env,
        exists=lambda path: exists_calls.append(path) or True,
        uid_factory=lambda: 1000,
    ):
        assert client.entered == 1

    assert exists_calls == []
    assert env == {"CONTAINER_HOST": "unix:///explicit.sock", "KEEP": "value"}
    assert client.exited == 1


def test_rootless_socket_precedes_rootful_socket_and_factory_sees_os_environ(
    podman_api: ModuleType, monkeypatch: pytest.MonkeyPatch
) -> None:
    client = FakeClient(FakeContainers())
    rootless = Path("/run/user/1200/podman/podman.sock")
    rootful = Path("/run/podman/podman.sock")
    monkeypatch.delenv("CONTAINER_HOST", raising=False)
    observed_paths: list[Path] = []

    def exists(path: Path) -> bool:
        observed_paths.append(path)
        return path in {rootless, rootful}

    def factory() -> FakeClient:
        assert os.environ["CONTAINER_HOST"] == f"unix://{rootless}"
        return client

    with podman_api.connect(
        "project-a",
        "dev",
        client_factory=factory,
        env=os.environ,
        exists=exists,
        uid_factory=lambda: 1200,
    ):
        pass

    assert observed_paths == [rootless]
    assert "CONTAINER_HOST" not in os.environ


def test_rootful_socket_is_used_when_rootless_socket_is_missing(
    podman_api: ModuleType,
) -> None:
    client = FakeClient(FakeContainers())
    env: dict[str, str] = {}
    rootless = Path("/run/user/1200/podman/podman.sock")
    rootful = Path("/run/podman/podman.sock")

    with podman_api.connect(
        "project-a",
        "dev",
        client_factory=lambda: client,
        env=env,
        exists=lambda path: path == rootful,
        uid_factory=lambda: 1200,
    ):
        assert env["CONTAINER_HOST"] == f"unix://{rootful}"

    assert "CONTAINER_HOST" not in env
    assert rootless != rootful


def test_missing_socket_raises_typed_error(podman_api: ModuleType) -> None:
    with pytest.raises(podman_api.PodmanSocketError) as raised:
        with podman_api.connect(
            "project-a",
            "dev",
            client_factory=lambda: pytest.fail("client factory must not be called"),
            env={},
            exists=lambda _: False,
            uid_factory=lambda: 1200,
        ):
            pass

    assert raised.value.paths == (
        Path("/run/user/1200/podman/podman.sock"),
        Path("/run/podman/podman.sock"),
    )


def test_podman_exception_is_wrapped_with_cause(podman_api: ModuleType) -> None:
    containers = FakeContainers()
    original = APIError("service unavailable")
    containers.error = original

    with _connect(podman_api, FakeClient(containers)) as api:
        with pytest.raises(podman_api.PodmanAPIError) as raised:
            api.find_container()

    assert raised.value.__cause__ is original


def test_exact_compose_project_and_service_labels_are_used(
    podman_api: ModuleType,
) -> None:
    container = FakeContainer()
    containers = FakeContainers((container,))

    with _connect(podman_api, FakeClient(containers)) as api:
        assert api.find_container() is container

    assert containers.calls == [
        {
            "all": True,
            "sparse": True,
            "filters": {
                "label": [
                    "io.podman.compose.project=project-a",
                    "io.podman.compose.service=dev",
                ]
            },
        }
    ]


@pytest.mark.parametrize(
    ("containers", "error_name"),
    [
        ((), "ContainerNotFoundError"),
        ((FakeContainer(), FakeContainer()), "AmbiguousContainerError"),
    ],
)
def test_health_wait_has_typed_container_cardinality_outcomes(
    podman_api: ModuleType,
    containers: Sequence[FakeContainer],
    error_name: str,
) -> None:
    with _connect(podman_api, FakeClient(FakeContainers(containers))) as api:
        with pytest.raises(getattr(podman_api, error_name)):
            api.wait_until_healthy(timeout=1.0)


def test_waits_until_exact_container_is_healthy(podman_api: ModuleType) -> None:
    clock = FakeClock()
    container = FakeContainer(("starting", "healthy"))

    with _connect(
        podman_api,
        FakeClient(FakeContainers((container,))),
        clock=clock,
        sleep=clock.sleep,
    ) as api:
        assert api.wait_until_healthy(timeout=1.0, interval=0.2) is container

    assert clock.sleeps == [0.2]


def test_unhealthy_container_raises_typed_error(podman_api: ModuleType) -> None:
    container = FakeContainer(("unhealthy",))

    with _connect(podman_api, FakeClient(FakeContainers((container,)))) as api:
        with pytest.raises(podman_api.ContainerUnhealthyError) as raised:
            api.wait_until_healthy(timeout=1.0, interval=0.2)

    assert raised.value.health == "unhealthy"


def test_health_wait_has_bounded_typed_timeout(podman_api: ModuleType) -> None:
    clock = FakeClock()
    container = FakeContainer(("starting",))

    with _connect(
        podman_api,
        FakeClient(FakeContainers((container,))),
        clock=clock,
        sleep=clock.sleep,
    ) as api:
        with pytest.raises(podman_api.ContainerHealthTimeoutError) as raised:
            api.wait_until_healthy(timeout=0.5, interval=0.2)

    assert raised.value.timeout == 0.5
    assert clock.sleeps == [0.2, 0.2, pytest.approx(0.1)]
    assert clock.now == pytest.approx(0.5)


def test_exec_is_noninteractive_demuxed_and_preserves_result(
    podman_api: ModuleType,
) -> None:
    container = FakeContainer()
    container.exec_result = (17, (b"kept stdout", b"kept stderr"))
    environment = {"GH_TOKEN": "per-exec-only"}

    with _connect(podman_api, FakeClient(FakeContainers((container,)))) as api:
        result = api.exec(
            ("task", "python:test", "--", "tests/automation"),
            environment=environment,
            workdir="/workspace",
        )

    assert result.exit_code == 17
    assert result.stdout == b"kept stdout"
    assert result.stderr == b"kept stderr"
    assert container.exec_calls == [
        (
            ["task", "python:test", "--", "tests/automation"],
            {
                "stdout": True,
                "stderr": True,
                "stdin": False,
                "tty": False,
                "privileged": False,
                "detach": False,
                "stream": False,
                "socket": False,
                "environment": environment,
                "workdir": "/workspace",
                "demux": True,
            },
        )
    ]


def test_exec_rejects_a_shell_command_string(podman_api: ModuleType) -> None:
    with _connect(podman_api, FakeClient(FakeContainers((FakeContainer(),)))) as api:
        with pytest.raises(TypeError, match="argument array"):
            api.exec("task python:test")


def test_module_has_no_subprocess_or_cli_fallback(podman_api: ModuleType) -> None:
    source_path = Path(podman_api.__file__ or "")
    tree = ast.parse(source_path.read_text())
    imported_modules = {
        alias.name
        for node in ast.walk(tree)
        if isinstance(node, ast.Import)
        for alias in node.names
    }
    imported_modules.update(
        node.module or "" for node in ast.walk(tree) if isinstance(node, ast.ImportFrom)
    )

    assert "subprocess" not in imported_modules
    assert "scripts.automation.process" not in imported_modules
