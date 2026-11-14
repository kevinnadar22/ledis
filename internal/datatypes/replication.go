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
	Host       string
	Port       string
	ReplID     string
	ReplOffset int64
}
