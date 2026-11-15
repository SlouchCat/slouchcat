/** @type {string[]} */
const physicalLegacy = [
  'src/app/AppLayout.module.css',
  'src/features/admin/AdminHomePage.module.css',
  'src/features/admin/backup/Backup.module.css',
  'src/features/admin/integrations/Integrations.module.css',
  'src/features/admin/item/AdminItem.module.css',
  'src/features/admin/stats/Stats.module.css',
  'src/features/catalog/DetailHeader.module.css',
  'src/features/catalog/HomePage.module.css',
  'src/features/catalog/ItemMeta.module.css',
  'src/features/catalog/LibraryGrid.module.css',
  'src/features/catalog/watched.module.css',
  'src/features/discover/CoverWatchlist.module.css',
  'src/features/player/Player.module.css',
  'src/ui/AdminPage.module.css',
  'src/ui/AdminSideNav.module.css',
  'src/ui/AdminSideNavMenu.module.css',
  'src/ui/AppHeader.module.css',
  'src/ui/AudioPlayer.module.css',
  'src/ui/Backdrop.module.css',
  'src/ui/BandPanel.module.css',
  'src/ui/BarChart.module.css',
  'src/ui/CandidateCard.module.css',
  'src/ui/ChapterCard.module.css',
  'src/ui/CollectionCard.module.css',
  'src/ui/DashRow.module.css',
  'src/ui/DataTable.module.css',
  'src/ui/DisclosureGroup.module.css',
  'src/ui/EpisodeCard.module.css',
  'src/ui/EpisodeListItem.module.css',
  'src/ui/FacetSelect.module.css',
  'src/ui/FolderList.module.css',
  'src/ui/HeroCarousel.module.css',
  'src/ui/ImageTile.module.css',
  'src/ui/LibraryTile.module.css',
  'src/ui/MiniPlayer.module.css',
  'src/ui/PlayerCard.module.css',
  'src/ui/PlayerChrome.module.css',
  'src/ui/PlayerPanel.module.css',
  'src/ui/PosterCard.module.css',
  'src/ui/ProfileTile.module.css',
  'src/ui/Row.module.css',
  'src/ui/SeekBar.module.css',
  'src/ui/SideNav.module.css',
  'src/ui/StatTile.module.css',
  'src/ui/StationTile.module.css',
  'src/ui/StreamTile.module.css',
  'src/ui/VolumeControl.module.css',
  'src/ui/VolumeRow.module.css',
  'src/ui/global.css',
];

/** @type {import('stylelint').Config} */
export default {
  extends: ['stylelint-config-standard'],
  plugins: ['stylelint-declaration-strict-value'],
  rules: {
    // Colours come only from design tokens (CLAUDE.md rule 9).
    'color-no-hex': true,
    'color-named': 'never',
    'scale-unlimited/declaration-strict-value': [
      [
        'color',
        'background-color',
        'border-color',
        'fill',
        'stroke',
        'outline-color',
        'box-shadow',
      ],
      {
        ignoreValues: ['currentcolor', 'transparent', 'inherit', 'initial', 'unset', 'none'],
        // Only var(--sc-...) is allowed; rgb(), hsl() and friends are rejected as well.
        ignoreFunctions: false,
      },
    ],
    // CSS Modules are accessed as styles.camelCase.
    'selector-class-pattern': [
      '^[a-z][a-zA-Z0-9]*$',
      { message: 'Expected class selector to be camelCase (CSS Modules)' },
    ],
    'selector-pseudo-class-no-unknown': [true, { ignorePseudoClasses: ['global', 'local'] }],
    // Logical properties instead of left and right (concept 22, 4.10): a right-to-left language
    // later must not have to touch every new rule. The files below predate the rule (stage 1 of
    // concept 22); they keep their physical properties until the right-to-left concept rebuilds
    // them, and no file joins the list.
    'property-disallowed-list': [
      [
        '/^(margin|padding|scroll-margin|scroll-padding)-(left|right)$/',
        '/^border-(left|right)(-.+)?$/',
        '/^border-(top|bottom)-(left|right)-radius$/',
        'left',
        'right',
      ],
      { message: 'Use the logical property (inline-start/end, inset-inline-*; concept 22, 4.10)' },
    ],
    'declaration-property-value-disallowed-list': [
      { 'text-align': ['left', 'right'], float: ['left', 'right'], clear: ['left', 'right'] },
      { message: 'Use start or end instead of left or right (concept 22, 4.10)' },
    ],
  },
  overrides: [
    {
      files: physicalLegacy,
      rules: {
        'property-disallowed-list': null,
        'declaration-property-value-disallowed-list': null,
      },
    },
  ],
};
