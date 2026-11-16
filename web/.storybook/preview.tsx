import '@slouchcat/design-tokens/tokens.css';
import '@/ui/global.css';
// The compact density of the administration (ADR 0020), as in the app (main.tsx).
import '@/ui/density.css';
// The administration's look (build plan 2.7): stories with the compact density show it.
import '@/ui/admin-look.css';
import '@/i18n';
import type { Preview } from '@storybook/react-vite';
import { clearUnlistedLanguage, setLanguage, supportedLanguages } from '@/i18n';
import { setPseudoLanguage } from '@/i18n/pseudo';
import { loadSpatialNavigation } from '@/ui';
// Straight from its module: the '@/ui/aria' barrel would load every stylesheet before the stories
// and change the order of the cascade.
import { AdminDensityScope } from '@/ui/adminDensity';

const preview: Preview = {
  parameters: {
    // The app is dark only; global.css paints the body. Storybook's own backgrounds stay off.
    backgrounds: { disable: true },
    layout: 'padded',
    controls: { disable: true },
    actions: { disable: true },
  },
  // Density of the story (concept 12, ADR 0020): `default` is today's size everywhere; a story
  // with `globals: { density: 'compact' }` renders the way an administration page does:
  // <html> carries data-shell="admin" like /admin, its root data-density="compact" (ui/density.css
  // maps the component tokens), and the AdminDensityScope puts the same attribute on the portals
  // of its dialogs and menus.
  globalTypes: {
    density: {
      description: 'Density of the administration (ADR 0020)',
      toolbar: { items: ['default', 'compact'] },
    },
    // Every shipped language for the visual check of a translation round (concept 22 section
    // 4.6, step 3; `?globals=locale:fi`), and en-XA, the pseudo language (section 4.4): every text
    // 40 % longer and in brackets, to find cut-off texts. The screenshot baselines stay German.
    locale: {
      description: 'Language of the story',
      toolbar: { items: [...supportedLanguages, 'en-XA'] },
    },
  },
  initialGlobals: { density: 'default', locale: 'de' },
  decorators: [
    (Story, context) => {
      const compact = context.globals.density === 'compact';
      if (compact) document.documentElement.dataset.shell = 'admin';
      else delete document.documentElement.dataset.shell;
      return compact ? (
        <div data-density="compact" style={{ display: 'contents' }}>
          <AdminDensityScope>
            <Story />
          </AdminDensityScope>
        </div>
      ) : (
        <Story />
      );
    },
  ],
  loaders: [
    async (context) => {
      // German like the e2e suite unless the toolbar asks for another language; spatial
      // components need norigin before they render.
      const locale = String(context.globals.locale ?? 'de');
      if (locale === 'en-XA') await setPseudoLanguage();
      else {
        clearUnlistedLanguage();
        await setLanguage(locale);
      }
      await loadSpatialNavigation();
      return {};
    },
  ],
};

export default preview;
