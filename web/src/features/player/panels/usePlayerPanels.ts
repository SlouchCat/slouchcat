import { useCallback, useRef, useState, type RefObject } from 'react';
import { PANEL_IDS } from './panelIds';
import type { SettingsPage } from './SettingsPanel';

/**
 * The panel beside the video: sound (T), subtitles (U), the gear (Q opens its quality page) or the
 * shortcut help (?). One at a time.
 */
export type PanelName = 'audio' | 'subtitle' | 'settings' | 'help';

/**
 * The panels of the online and the offline video player: which one is open, the page of the gear
 * (`direct` when a key opened it on that page, with no way up), the props of the sound and
 * subtitle buttons in the bar, and the button of the bar each panel opens above (concept 23 P-2:
 * `anchors`, also when a key opened the panel; `anchor` is the open panel's, for PlayerPanelAnchor).
 */
export function usePlayerPanels() {
  const [panel, setPanel] = useState<PanelName | null>(null);
  const audioAnchor = useRef<HTMLButtonElement>(null);
  const subtitleAnchor = useRef<HTMLButtonElement>(null);
  const settingsAnchor = useRef<HTMLButtonElement>(null);
  const helpAnchor = useRef<HTMLButtonElement>(null);
  const anchors: Record<PanelName, RefObject<HTMLButtonElement | null>> = {
    audio: audioAnchor,
    subtitle: subtitleAnchor,
    settings: settingsAnchor,
    help: helpAnchor,
  };
  const [settingsView, setSettingsView] = useState<{ page: SettingsPage; direct: boolean }>({
    page: 'main',
    direct: false,
  });
  const openPanel = useCallback((name: PanelName) => setPanel(name), []);
  const togglePanel = useCallback((name: PanelName) => {
    setPanel((current) => (current === name ? null : name));
  }, []);
  const closePanel = useCallback(() => setPanel(null), []);
  const openSettings = useCallback((page: SettingsPage, direct: boolean) => {
    setSettingsView({ page, direct });
    setPanel('settings');
  }, []);
  /** The sound or subtitle button of the bar: it opens and closes its panel. */
  const panelButton = (
    name: 'audio' | 'subtitle',
    label: string,
    text: string,
    isDisabled: boolean,
  ) => ({
    label,
    text,
    expanded: panel === name,
    controls: PANEL_IDS[name],
    ref: anchors[name],
    onPress: () => togglePanel(name),
    isDisabled,
  });
  return {
    panel,
    openPanel,
    togglePanel,
    closePanel,
    settingsView,
    setSettingsView,
    openSettings,
    panelButton,
    anchors,
    anchor: panel === null ? null : anchors[panel],
  };
}
