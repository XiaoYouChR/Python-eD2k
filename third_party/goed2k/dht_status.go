package goed2k

type DHTStatus struct {
	Bootstrapped      bool
	Firewalled        bool
	LiveNodes         int
	ReplacementNodes  int
	RouterNodes       int
	RunningTraversals int
	KnownNodes        int
	InitialBootstrap  bool
	ListenPort        int
	StoragePoint      string
}
