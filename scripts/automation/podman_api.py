"""Typed Podman API access for the host development launcher."""

from __future__ import annotations

import os
import time
from collections.abc import Callable, Iterator, Mapping, MutableMapping, Sequence
from contextlib import contextmanager
from dataclasses import dataclass
from pathlib import Path
from typing import Protocol, cast

from podman import PodmanClient
from podman.errors import APIError, PodmanError

CONTAINER_HOST = "CONTAINER_HOST"
ROOTFUL_SOCKET = Path("/run/podman/podman.sock")
DEFAULT_API_TIMEOUT_SECONDS = 30
PODMAN_EXCEPTIONS = (APIError, PodmanError)


class ContainerProtocol(Protocol):
    """Podman container behavior used by the launcher."""

    @property
    def name(self) -> str: ...

    @property
    def attrs(self) -> Mapping[str, object]: ...

    def reload(self, **kwargs: object) -> None: ...

    def exec_run(self, arguments: list[str], **kwargs: object) -> object: ...


class ContainersProtocol(Protocol):
    """Podman container collection behavior used by the launcher."""

    def list(self, **kwargs: object) -> Sequence[ContainerProtocol]: ...


class APIClientProtocol(Protocol):
    """Podman transport behavior used to bound individual API requests."""

    timeout: float | None


class ClientProtocol(Protocol):
    """Context-managed Podman client behavior used by the launcher."""

    containers: ContainersProtocol
    api: APIClientProtocol

    def __enter__(self) -> ClientProtocol: ...

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc_value: BaseException | None,
        traceback: object,
    ) -> None: ...


ClientFactory = Callable[[], ClientProtocol]
PathExists = Callable[[Path], bool]
Clock = Callable[[], float]
Sleeper = Callable[[float], None]


class PodmanAPIError(RuntimeError):
    """PodmanAPIError is the typed boundary for Podman service failures."""


class PodmanSocketError(PodmanAPIError):
    """PodmanSocketError reports that no supported Podman socket exists."""

    def __init__(self, paths: tuple[Path, ...]) -> None:
        self.paths = paths
        checked = ", ".join(str(path) for path in paths)
        super().__init__(f"Podman socket not found; checked: {checked}")


class ContainerNotFoundError(PodmanAPIError):
    """ContainerNotFoundError reports a missing exact Compose service container."""


class AmbiguousContainerError(PodmanAPIError):
    """AmbiguousContainerError reports multiple exact Compose service containers."""


class ContainerUnhealthyError(PodmanAPIError):
    """ContainerUnhealthyError reports an explicit unhealthy health-check state."""

    def __init__(self, project: str, service: str, health: str) -> None:
        self.health = health
        super().__init__(f"Compose container {project}/{service} is {health}")


class ContainerNotRunningError(PodmanAPIError):
    """ContainerNotRunningError reports a container that stopped during readiness."""

    def __init__(self, project: str, service: str, state: str) -> None:
        self.state = state
        super().__init__(f"Compose container {project}/{service} is {state}, not running")


class ContainerHealthTimeoutError(PodmanAPIError):
    """ContainerHealthTimeoutError reports a bounded health-wait timeout."""

    def __init__(self, project: str, service: str, timeout: float) -> None:
        self.timeout = timeout
        super().__init__(
            f"Compose container {project}/{service} was not healthy after {timeout:g} seconds"
        )


@dataclass(frozen=True, slots=True)
class ExecResult:
    """ExecResult preserves the exit code and demultiplexed command output."""

    exit_code: int
    stdout: bytes
    stderr: bytes


@dataclass(frozen=True, slots=True)
class ContainerStatus:
    """ContainerStatus is the exact container name and inspected runtime state."""

    name: str
    state: str


@dataclass(slots=True)
class PodmanAPI:
    """PodmanAPI performs exact Compose lookup, readiness, and exec operations."""

    client: ClientProtocol
    project: str
    service: str
    clock: Clock
    sleep: Sleeper

    def find_container(self) -> ContainerProtocol:
        """Return the one container matching exact Compose project and service labels."""
        try:
            containers = self._list_containers(DEFAULT_API_TIMEOUT_SECONDS)
        except PODMAN_EXCEPTIONS as error:
            raise PodmanAPIError("Podman container lookup failed") from error
        return self._select_container(containers)

    def status(self) -> ContainerStatus:
        """Reload and report the exact container without sparse-object shortcuts."""
        container = self.find_container()
        try:
            with self._transport_timeout(DEFAULT_API_TIMEOUT_SECONDS):
                container.reload()
        except PODMAN_EXCEPTIONS as error:
            raise PodmanAPIError("Podman container inspection failed") from error
        return ContainerStatus(container.name, _container_state(container.attrs))

    def wait_until_healthy(self, *, timeout: float, interval: float = 0.2) -> ContainerProtocol:
        """Wait within a deadline for the exact Compose container to become healthy."""
        if timeout <= 0:
            raise ValueError("health timeout must be positive")
        if interval <= 0:
            raise ValueError("health polling interval must be positive")

        deadline = self.clock() + timeout
        while True:
            remaining = self._remaining(deadline, timeout)
            try:
                containers = self._list_containers(remaining)
            except PODMAN_EXCEPTIONS as error:
                if self.clock() >= deadline:
                    raise ContainerHealthTimeoutError(
                        self.project, self.service, timeout
                    ) from error
                raise PodmanAPIError("Podman container lookup failed") from error
            container = self._select_container(containers)

            remaining = self._remaining(deadline, timeout)
            try:
                with self._transport_timeout(remaining):
                    container.reload()
            except PODMAN_EXCEPTIONS as error:
                if self.clock() >= deadline:
                    raise ContainerHealthTimeoutError(
                        self.project, self.service, timeout
                    ) from error
                raise PodmanAPIError("Podman container inspection failed") from error

            self._remaining(deadline, timeout)
            attrs = container.attrs
            state = _container_state(attrs)
            if state != "running":
                raise ContainerNotRunningError(self.project, self.service, state)
            health = _health_status(attrs)
            if health == "healthy":
                return container
            if health == "unhealthy":
                raise ContainerUnhealthyError(self.project, self.service, health)

            remaining = self._remaining(deadline, timeout)
            self.sleep(min(interval, remaining))

    def exec(
        self,
        arguments: Sequence[str],
        *,
        environment: Mapping[str, str] | None = None,
        workdir: str = "/workspace",
    ) -> ExecResult:
        """Execute a noninteractive argument array and preserve its complete result."""
        if isinstance(arguments, (str, bytes)):
            raise TypeError("exec requires an argument array, not a command string")
        normalized = list(arguments)
        if not normalized:
            raise ValueError("exec argument array must not be empty")
        if not all(isinstance(argument, str) for argument in normalized):
            raise TypeError("exec argument array must contain only strings")

        container = self.find_container()
        try:
            with self._transport_timeout(DEFAULT_API_TIMEOUT_SECONDS):
                raw_result = container.exec_run(
                    normalized,
                    stdout=True,
                    stderr=True,
                    stdin=False,
                    tty=False,
                    privileged=False,
                    detach=False,
                    stream=False,
                    socket=False,
                    environment=dict(environment or {}),
                    workdir=workdir,
                    demux=True,
                )
        except PODMAN_EXCEPTIONS as error:
            raise PodmanAPIError("Podman container exec failed") from error

        exit_code, output = _exec_response(raw_result)
        stdout, stderr = output
        return ExecResult(exit_code, _output_bytes(stdout), _output_bytes(stderr))

    def _list_containers(self, timeout: float) -> Sequence[ContainerProtocol]:
        filters = [
            f"label=io.podman.compose.project={self.project}",
            f"label=io.podman.compose.service={self.service}",
        ]
        with self._transport_timeout(timeout):
            return self.client.containers.list(all=False, sparse=True, filters=filters)

    def _select_container(self, containers: Sequence[ContainerProtocol]) -> ContainerProtocol:
        if not containers:
            raise ContainerNotFoundError(
                f"Compose container {self.project}/{self.service} was not found"
            )
        if len(containers) != 1:
            raise AmbiguousContainerError(
                f"multiple Compose containers match {self.project}/{self.service}"
            )
        return containers[0]

    def _remaining(self, deadline: float, timeout: float) -> float:
        remaining = deadline - self.clock()
        if remaining <= 0:
            raise ContainerHealthTimeoutError(self.project, self.service, timeout)
        return remaining

    @contextmanager
    def _transport_timeout(self, timeout: float) -> Iterator[None]:
        previous = self.client.api.timeout
        bounded = timeout if previous is None or previous <= 0 else min(previous, timeout)
        self.client.api.timeout = bounded
        try:
            yield
        finally:
            self.client.api.timeout = previous


def _container_state(attrs: Mapping[str, object]) -> str:
    state = attrs.get("State")
    if not isinstance(state, Mapping):
        return "unknown"
    status = state.get("Status")
    return status if isinstance(status, str) else "unknown"


def _health_status(attrs: Mapping[str, object]) -> str:
    state = attrs.get("State")
    if not isinstance(state, Mapping):
        return "unknown"
    health = state.get("Health")
    if not isinstance(health, Mapping):
        return "unknown"
    status = health.get("Status")
    return status if isinstance(status, str) else "unknown"


def _exec_response(raw_result: object) -> tuple[int, tuple[object, object]]:
    if not isinstance(raw_result, tuple) or len(raw_result) != 2:
        raise PodmanAPIError("Podman exec returned an invalid response")
    exit_code, output = raw_result
    if not isinstance(exit_code, int):
        raise PodmanAPIError("Podman exec did not return an exit code")
    if not isinstance(output, tuple) or len(output) != 2:
        raise PodmanAPIError("Podman exec did not return demultiplexed output")
    return exit_code, output


def _output_bytes(output: object) -> bytes:
    if output is None:
        return b""
    if isinstance(output, bytes):
        return output
    raise PodmanAPIError("Podman exec returned non-byte output")


def _container_host(
    env: Mapping[str, str], *, exists: PathExists, uid_factory: Callable[[], int]
) -> str:
    explicit = env.get(CONTAINER_HOST)
    if explicit:
        return explicit

    paths = (
        Path(f"/run/user/{uid_factory()}/podman/podman.sock"),
        ROOTFUL_SOCKET,
    )
    for path in paths:
        if exists(path):
            return f"unix://{path}"
    raise PodmanSocketError(paths)


@contextmanager
def connect(
    project: str,
    service: str,
    *,
    client_factory: ClientFactory | None = None,
    exists: PathExists = Path.exists,
    env: MutableMapping[str, str] = os.environ,
    uid_factory: Callable[[], int] = os.getuid,
    clock: Clock = time.monotonic,
    sleep: Sleeper = time.sleep,
) -> Iterator[PodmanAPI]:
    """Open a typed Podman API context using the selected Unix socket."""
    host = _container_host(env, exists=exists, uid_factory=uid_factory)
    previous = env.get(CONTAINER_HOST)
    env[CONTAINER_HOST] = host
    try:
        try:
            if client_factory is None:
                client_context = cast(
                    ClientProtocol,
                    PodmanClient.from_env(
                        environment=dict(env), timeout=DEFAULT_API_TIMEOUT_SECONDS
                    ),
                )
            else:
                client_context = client_factory()
            with client_context as client:
                yield PodmanAPI(client, project, service, clock, sleep)
        except PODMAN_EXCEPTIONS as error:
            raise PodmanAPIError("Podman client connection failed") from error
    finally:
        if previous is None:
            env.pop(CONTAINER_HOST, None)
        else:
            env[CONTAINER_HOST] = previous
