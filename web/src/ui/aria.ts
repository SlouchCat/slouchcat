// react-aria based UI for forms, dialogs and the player, kept apart from '@/ui' so the browsing
// pages (start, detail) never load react-aria (P-10). Features import it only from '@/ui/aria'.
// The table, the chart, the storage meter, the command block, the counter
// tiles and the link cards of the admin, settings and downloads pages live here too: they carry
// CSS, and anything in '@/ui' lands in the start bundle. So do the administration's sidebar and
// the overview's cards and rows: the app shell loads its admin column lazily (React.lazy), while
// `data-shell="admin"` reserves the column's width before the first paint.
export { AriaButton } from './AriaButton';
export { AccentPicker } from './AccentPicker';
export { ActivityFeed, type ActivityEntry, type ActivityTone } from './ActivityFeed';
export { DashCard } from './DashCard';
export { StreamRow } from './StreamRow';
export { TaskRow } from './TaskRow';
export {
  AdminSideNav,
  type AdminSideNavEntry,
  type AdminSideNavGroup,
  type AdminSideNavProps,
  type AdminSideNavTone,
} from './AdminSideNav';
export { AdminSideNavMenu, type AdminSideNavMenuProps } from './AdminSideNavMenu';
export { BarChart, type BarDatum } from './BarChart';
export { CheckboxGroup } from './CheckboxGroup';
export { CodeBlock } from './CodeBlock';
export { DataTable, type DataColumn } from './DataTable';
export { ConfirmDialog, ConfirmInputDialog, FormDialog } from './Dialog';
export { EditorDialog } from './EditorDialog';
export {
  FolderList,
  contentsOf,
  defaultProviders,
  folderListError,
  newFolderEntry,
  type FolderEntry,
} from './FolderList';
export { Form } from './Form';
export { LinkCard } from './LinkCard';
export { LockToggle } from './LockToggle';
export { MenuButton } from './Menu';
export { MessageContent, MessageDialog } from './MessageDialog';
export { PathList, type PathItem } from './PathList';
export { PinField } from './PinField';
export {
  ProviderList,
  allowedProviders,
  canIdentify,
  providerChain,
  providerListError,
} from './ProviderList';
export { RadioGroup } from './RadioGroup';
export { SelectCheck } from './SelectCheck';
export { StatTile } from './StatTile';
export { StorageMeter, type StoragePart } from './StorageMeter';
export { Switch } from './Switch';
export { Tabs } from './Tabs';
export { TextField } from './TextField';
export { TrailerDialog } from './TrailerDialog';
// The radio's «Jetzt läuft» (concept 16, 10.3), loaded when the mini player opens it.
export { NowPlayingDialog } from './NowPlayingDialog';
// The radio page's filters (concept 16, 10.1): a facet with counts, the phone's sheet.
export { FacetOptions, FacetSelect, type FacetOption } from './FacetSelect';
export { FilterSheet } from './FilterSheet';
export { AudioPlayer, type AudioPlayerMenu } from './AudioPlayer';
// Building blocks of the player (concept 13, 11.10).
export { ChapterCard } from './ChapterCard';
export { PlayerCard } from './PlayerCard';
export {
  OptionLinkRow,
  OptionList,
  OptionMoreRow,
  OptionRow,
  OptionSection,
  type OptionMark,
} from './OptionRow';
export {
  PlayerChrome,
  playerEndTime,
  playerRemainingMs,
  type PlayerChromePanelButton,
  type PlayerChromeLayout,
  type PlayerChromeStep,
  type PlayerChromeTab,
} from './PlayerChrome';
export { PlayerPanel } from './PlayerPanel';
export { SeekBar, type SeekBarChapter, type SeekBarSegment } from './SeekBar';
export { BOOST_MAX, clampVolume, VOLUME_STEP, VolumeControl } from './VolumeControl';
// Building blocks of the administration pages (concept 12, 11.2).
export { ActionMenu, type ActionMenuItem } from './ActionMenu';
export { AdminDensityScope, useDensityPortalProps } from './adminDensity';
export { AdminPage } from './AdminPage';
export { BandPanel } from './BandPanel';
export { type DataTableExpand } from './DataTable';
export { DisclosureGroup } from './DisclosureGroup';
export { HeatGrid } from './HeatGrid';
export { ImageTile } from './ImageTile';
export { MasterDetail, type MasterDetailItem } from './MasterDetail';
export { RankedChoiceList, type RankedChoice } from './RankedChoiceList';
export { RankHint, RankSelect, rankTo, type RankMode } from './RankSelect';
export { SaveBar } from './SaveBar';
export { SectionHeader } from './SectionHeader';
export { SettingRow } from './SettingRow';
export { StatusStrip } from './StatusStrip';
export { StreamTile, type StreamFact } from './StreamTile';
export { type TabItem } from './Tabs';
export { useDrafts, type Drafts } from './useDrafts';
export { useSearchSelection } from './useSearchSelection';
export { useUnsavedChanges, type UnsavedChangesDialog } from './useUnsavedChanges';

export { afterDialogsClosed, afterRender, focusLost, focusTitleIfLost } from './focusAfter';
export { focusOnMount } from './focusOnMount';
export { StatusRegion, useStatusMessage, type StatusMessage } from './StatusRegion';
