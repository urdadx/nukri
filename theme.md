# Theme Implementation Notes

## Sidebar Icon Colors

The current theme format already provides enough colors for sidebar icons. We do not need separate colors for Home, Downloads, Trash, and every other place unless more granular customization is added later.

Recommended rendering rules:

- Normal icon and text: `sidebar_fg`
- Sidebar background: `sidebar_bg`
- Section headings such as Devices: `sidebar_title`
- Selected icon and text: `sidebar_item_selected_fg`
- Selected row background: `sidebar_item_selected_bg`
- Broken symlink icon: `error`
- Borders and dividers: `sidebar_border`, `sidebar_border_active`, and `sidebar_divider`
- File-panel directory icons: `directory_icon_color`

Example behavior:

```text
Normal Downloads row
  icon: sidebar_fg
  text: sidebar_fg
  background: sidebar_bg

Selected Downloads row
  icon: sidebar_item_selected_fg
  text: sidebar_item_selected_fg
  background: sidebar_item_selected_bg

Broken linked place
  icon: error
  text: sidebar_fg
```

## Architecture

1. Parse the theme TOML into a `Theme` struct.
2. Keep `SidebarItem` semantic, containing values such as `Kind`, `Icon`, and `Path` rather than resolved colors.
3. Apply theme colors in the sidebar renderer according to the row's semantic kind and selection state.
4. Preserve broken-link state in `SidebarItem`, or return it from place resolution, so the renderer can use the theme's `error` color.
5. Treat empty colors such as `directory_icon_color = ""` as instructions to inherit the relevant foreground color.

The icons package should own glyphs, while the theme should own colors. Sidebar colors should not be hardcoded into `icons.Style`, because that would couple icons to one theme and make runtime theme switching harder.
