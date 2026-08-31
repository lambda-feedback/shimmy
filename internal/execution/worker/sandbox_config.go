package worker

// SandboxConfig holds the configuration for nsjail-based process sandboxing.
// Zero values mean "disabled" / "unlimited". Sandboxing is Linux-only.
type SandboxConfig struct {
	// Enabled activates nsjail wrapping for worker processes.
	Enabled bool `conf:"enabled"`

	// NsjailPath is the path to the nsjail binary. Default: /usr/sbin/nsjail.
	NsjailPath string `conf:"nsjail_path"`

	// User is the uid:gid the worker runs as inside the sandbox.
	// Default: "65534:65534" (nobody:nogroup).
	User string `conf:"user"`

	// ReadOnlyBinds are host paths bind-mounted read-only at the same path
	// inside the sandbox. E.g. ["/usr", "/lib", "/lib64"].
	ReadOnlyBinds []string `conf:"ro_binds"`

	// WritableBinds are host paths bind-mounted read-write at the same path
	// inside the sandbox. Required for file-mode: include "/tmp/shimmy".
	WritableBinds []string `conf:"rw_binds"`

	// TmpfsMounts are paths inside the sandbox to mount as tmpfs.
	TmpfsMounts []string `conf:"tmpfs"`

	// CpuTimeLimit is the CPU time limit in seconds. 0 = unlimited.
	CpuTimeLimit int `conf:"cpu_time_limit"`

	// MemoryLimit is the address-space limit in megabytes. 0 = unlimited.
	MemoryLimit int `conf:"memory_limit"`

	// MaxFds is the maximum number of open file descriptors. 0 = nsjail default.
	MaxFds int `conf:"max_fds"`

	// DisableNetwork removes network access inside the sandbox.
	DisableNetwork bool `conf:"disable_network"`

	// DisableCloneNewpid keeps the worker in the host PID namespace. Some
	// constrained hosts (rootless Podman, locked-down Fargate) reject a nested
	// PID namespace, which surfaces as "pthread_create ... Invalid argument".
	DisableCloneNewpid bool `conf:"disable_clone_newpid"`

	// DisableCloneNewipc keeps the worker in the host IPC namespace.
	DisableCloneNewipc bool `conf:"disable_clone_newipc"`

	// DisableCloneNewuts keeps the worker in the host UTS namespace.
	DisableCloneNewuts bool `conf:"disable_clone_newuts"`

	// DisableCloneNewcgroup keeps the worker in the host cgroup namespace.
	DisableCloneNewcgroup bool `conf:"disable_clone_newcgroup"`

	// CloneNewuser controls the user namespace: "" / "auto" drops it only when
	// shimmy runs as uid 0 (the usual container case, where nested CLONE_NEWUSER
	// is blocked); "enabled" always keeps it (rootless Podman needs it);
	// "disabled" always drops it.
	CloneNewuser string `conf:"clone_newuser"`

	// SeccompPolicyFile is the path to a kafel seccomp policy file, passed to
	// nsjail as --seccomp_policy. Empty = no seccomp filtering (nsjail still
	// sets NO_NEW_PRIVS by default). Mutually exclusive with SeccompString.
	SeccompPolicyFile string `conf:"seccomp_policy_file"`

	// SeccompString is an inline kafel seccomp policy, passed to nsjail as
	// --seccomp_string. Mutually exclusive with SeccompPolicyFile.
	SeccompString string `conf:"seccomp_string"`

	// Verbose lets nsjail log to stderr at its default verbosity. When false
	// (the default) nsjail runs with --quiet: only warnings, errors and fatal
	// cmdline/namespace failures reach stderr.
	Verbose bool `conf:"verbose"`
}
