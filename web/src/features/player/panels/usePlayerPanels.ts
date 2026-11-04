import { useCallback, useState } from 'react';
import { PANEL_IDS } from './panelIds';
import type { SettingsPage } from './SettingsPanel';

/**
 * The panel beside the video: sound (T), subtitles (U), the gear (Q opens its quality page) or the
 * shortcut help (?). One at a time.
 */
export type PanelName = 'audio' | 'subtitle' | 'settings' | 'help';

/**
 * The panels of the online and the offline video player: which one is open, the page of the gear
 * (`direct` when a key opened it on that page, with no way up) and the props of the sound and
 * subtitle buttons in the bar.
 */
export function usePlayerPanels() {
  const [panel, setPanel] = useState<PanelName | null>(null);
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
  };
}
