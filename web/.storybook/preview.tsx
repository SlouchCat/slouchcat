import '@slouchcat/design-tokens/tokens.css';
import '@/ui/global.css';
import '@/i18n';
import type { Preview } from '@storybook/react-vite';
import { setLanguage } from '@/i18n';
import { loadSpatialNavigation } from '@/ui';

const preview: Preview = {
  parameters: {
    // The app is dark only; global.css paints the body. Storybook's own backgrounds stay off.
    backgrounds: { disable: true },
    layout: 'padded',
    controls: { disable: true },
    actions: { disable: true },
  },
  loaders: [
    async () => {
      // German like the e2e suite; spatial components need norigin before they render.
      await setLanguage('de');
      await loadSpatialNavigation();
      return {};
    },
  ],
};

export default preview;
