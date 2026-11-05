/**
 * The ids of the player's panels: the button that opens one names it in `aria-controls` while it
 * is open (one panel of each kind at a time).
 */
export const PANEL_IDS = {
  audio: 'player-panel-audio',
  subtitle: 'player-panel-subtitle',
  settings: 'player-panel-settings',
  help: 'player-panel-help',
} as const;
