package clustermgmt

// Cluster is the reviewed listClusters projection used by Terraform state mapping.
type Cluster struct {
	ExtID                  *string   `json:"extId"`
	Name                   *string   `json:"name"`
	Categories             *[]string `json:"categories"`
	VMCount                *int64    `json:"vmCount"`
	InefficientVMCount     *int64    `json:"inefficientVmCount"`
	ContainerName          *string   `json:"containerName"`
	ClusterProfileExtID    *string   `json:"clusterProfileExtId"`
	BackupEligibilityScore *int64    `json:"backupEligibilityScore"`
}
