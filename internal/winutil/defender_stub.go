//go:build !windows

package winutil

// DefenderExclusionStatus represents the current state of Windows Defender exclusions.
type DefenderExclusionStatus struct {
	ExeExcluded  bool
	DirExcluded  bool
	PortExcluded bool
	Error        error
}

// CheckDefenderExclusions is a no-op on non-Windows platforms.
func CheckDefenderExclusions(exePath, dataDir string, port int) DefenderExclusionStatus {
	return DefenderExclusionStatus{}
}

// AddDefenderExclusions is a no-op on non-Windows platforms.
func AddDefenderExclusions(exePath, dataDir string) error {
	return nil
}

// RemoveDefenderExclusions is a no-op on non-Windows platforms.
func RemoveDefenderExclusions(exePath, dataDir string) error {
	return nil
}

// PromptDefenderExclusion is a no-op on non-Windows platforms.
func PromptDefenderExclusion(dataDir string) error {
	return nil
}

// HandleDefenderExclusionFlag is a no-op on non-Windows platforms.
func HandleDefenderExclusionFlag(args []string) bool {
	return false
}
