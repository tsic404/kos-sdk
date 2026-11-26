package manifest

// Permission is one name from the fixed application permission set of KOSD-C-75.
// The contract names five permissions (通知、剪贴板读、剪贴板写、文件选择、外部链接);
// these values are their wire form in an App manifest, identical to the names the
// console accepts and sends back in the handshake.
type Permission string

const (
	PermissionNotifications  Permission = "notifications"
	PermissionClipboardRead  Permission = "clipboard-read"
	PermissionClipboardWrite Permission = "clipboard-write"
	PermissionFilePicker     Permission = "file-picker"
	PermissionExternalLink   Permission = "external-link"
)

// AppPermissions is the fixed set, in contract order.
var AppPermissions = []Permission{
	PermissionNotifications,
	PermissionClipboardRead,
	PermissionClipboardWrite,
	PermissionFilePicker,
	PermissionExternalLink,
}

func isAppPermission(name string) bool {
	for _, permission := range AppPermissions {
		if string(permission) == name {
			return true
		}
	}
	return false
}
