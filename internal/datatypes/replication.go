package datatypes

type Role int

const (
	RoleMaster Role = iota
	RoleReplica
)

type ReplicationClient struct {
	ListeningPort int
	Capabilities  map[string]bool
	Queue         chan Command
}

type UpstreamMaster struct {
	ReplID     string
	ReplOffset int64
}
