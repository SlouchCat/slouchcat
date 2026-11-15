import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { CommandPalette, type CommandPaletteItem } from '@/ui/aria';
import { adminAreaIcons, adminAreas, firstSectionOf, sectionsOf } from './sections';

/**
 * The search palette of the administration (build plan 2.4), opened by «Bereich suchen» in the
 * header and by Ctrl+K / ⌘K (AppLayout): the overview, every area, every page with its area,
 * back to the app and the media search. Loaded with the administration's bundle, not the start
 * bundle (P-10). Mounted only while open.
 */
export function AdminFinder({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const areaDetail = t('admin.finder.area');
  const items: CommandPaletteItem[] = [
    { id: 'overview', title: t('admin.nav.overview'), icon: 'grid', href: paths.admin },
    ...adminAreas.map((area) => ({
      id: `area-${area}`,
      title: t(`admin.nav.areas.${area}`),
      detail: areaDetail,
      icon: adminAreaIcons[area],
      href: firstSectionOf(area).href,
    })),
    ...adminAreas.flatMap((area) =>
      sectionsOf(area).map((section) => ({
        id: `page-${section.key}`,
        title: t(`admin.sections.${section.key}.title`),
        detail: t(`admin.nav.areas.${area}`),
        icon: section.icon,
        href: section.href,
      })),
    ),
    { id: 'back', title: t('admin.nav.back'), icon: 'back', href: paths.home },
    { id: 'search', title: t('admin.finder.search'), icon: 'search', href: paths.search },
  ];
  return (
    <CommandPalette
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      label={t('admin.finder.label')}
      placeholder={t('admin.finder.placeholder')}
      resultsLabel={t('admin.finder.results')}
      emptyLabel={t('admin.finder.empty')}
      hint={t('admin.finder.hint')}
      items={items}
    />
  );
}
