package core

type SidebarItemKind int

const (
	Home SidebarItemKind = iota
	Desktop
	Documents
	Downloads
	Pictures
	Music
	Videos
	Root
	Trash
	Custom
	Device
)

type SidebarItem struct {
	Kind         SidebarItemKind
	Title        string
	Icon         string
	IdentityPath string
	Path         string
	Removable    bool
}

func NewSidebarItem(kind SidebarItemKind, title, icon, path, identityPath string) SidebarItem {
	return SidebarItem{
		Kind:         kind,
		Title:        title,
		Icon:         icon,
		IdentityPath: identityPath,
		Path:         path,
		Removable:    false,
	}
}

type SidebarRow struct {
	Section string
	Item    *SidebarItem
}

func (r SidebarRow) GetItem() *SidebarItem {
	return r.Item
}
