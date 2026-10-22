// react-aria based UI for forms, dialogs and the player, kept apart from '@/ui' so the browsing
// pages (start, detail) never load react-aria (P-10). Features import it only from '@/ui/aria'.
// The table, the chart, the storage meter, the admin navigation, the command block, the counter
// tiles and the link cards of the admin, settings and downloads pages live here too: they carry
// CSS, and anything in '@/ui' lands in the start bundle.
export { AriaButton } from './AriaButton';
export { AccentPicker } from './AccentPicker';
export { AdminNav, type AdminNavGroup, type AdminNavLink } from './AdminNav';
export { BarChart, type BarDatum } from './BarChart';
export { CheckboxGroup } from './CheckboxGroup';
export { CodeBlock } from './CodeBlock';
export { DataTable, type DataColumn } from './DataTable';
export { ConfirmDialog, FormDialog } from './Dialog';
export { LinkCard } from './LinkCard';
export { MenuButton } from './Menu';
export { MessageDialog } from './MessageDialog';
export { PinField } from './PinField';
export { RadioGroup } from './RadioGroup';
export { SelectableList, type SelectableItem } from './SelectableList';
export { StatTile } from './StatTile';
export { StorageMeter, type StoragePart } from './StorageMeter';
export { Switch } from './Switch';
export { Tabs } from './Tabs';
export { TextField } from './TextField';
export { TrailerDialog } from './TrailerDialog';
export { AudioPlayer, type AudioPlayerMenu } from './AudioPlayer';
