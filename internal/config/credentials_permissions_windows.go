//go:build windows

package config

import (
	"os"

	"golang.org/x/sys/windows"
)

// Existing Windows ACLs are left intact on reads. Rewrites replace the file
// with one protected by the current-user ACL below.
func repairCredentialPermissions(string) error {
	return nil
}

func secureCredentialDirectory(path string) error {
	return setCurrentUserOnlyACL(path)
}

func secureCredentialFile(path string) error {
	return setCurrentUserOnlyACL(path)
}

func setCurrentUserOnlyACL(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if info.IsDir() {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       inheritance,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	)
}
