import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { paths } from '@/app/paths';
import { Notice, TextLink } from '@/ui';
import {
  AdminPage,
  ConfirmDialog,
  StatusStrip,
  Tabs,
  useSearchSelection,
  useUnsavedChanges,
} from '@/ui/aria';
import {
  RADIO_RELAYS_POLL_MS,
  radioCatalogQuery,
  radioConfigQuery,
  radioOfferQuery,
  radioRelaysQuery,
  type RadioOfferStep,
} from './queries';
import { OfferTab } from './radio/OfferTab';
import { OverviewTab } from './radio/OverviewTab';
import { OwnStationsTab } from './radio/OwnStationsTab';
import { SettingsTab } from './radio/SettingsTab';
import { StationEditor } from './radio/StationEditor';
import { StationsTab } from './radio/StationsTab';

const route = getRouteApi('/_authed/_active/_chrome/admin/radio');

/**
 * The tabs (R-16, concept 16 stage 2); the ids are part of links and stay stable. «Einordnung» is
 * gone with the tag rules, «Angebot» is new (contract 1.28.0, concept 19 part B, B9.1).
 */
const TABS = ['uebersicht', 'angebot', 'sender', 'eigene', 'einstellungen'] as const;

/**
 * Webradio in the administration (concept 16, 11; variant A «Reiter»): the counters, a warning
 * without RAM buffer, the tabs «Übersicht» (catalog, offer, live relays, server load), «Angebot»
 * (the stations the app shows, concept 19 part B), «Sender» (the offer or the whole catalog, the
 * editor in `?item=`), «Eigene Sender» and «Einstellungen». The tab is `?tab=` (S-14). The draft
 * of «Angebot» lives in the page, so it stays across the tabs (S-17), and leaving the page with
 * it asks first (S-13).
 */
export function RadioAdminPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.radio.title'));
  const session = useActiveSession();
  if (session.account.role !== 'admin' || !session.profile.isPrimary) {
    return (
      <AdminPage title={t('admin.radio.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <RadioAdminContent />;
}

function RadioAdminContent() {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const percent = useMemo(
    () => new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }),
    [i18n.language],
  );
  const titleRef = useRef<HTMLHeadingElement>(null);
  const [tab, selectTab] = useSearchSelection('tab', TABS, 'uebersicht', { normalise: true });
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const editing = search.item !== undefined ? Number(search.item) : undefined;
  const openEditor = (id: number | undefined) =>
    void navigate({
      search: (previous) => ({ ...previous, item: id === undefined ? undefined : String(id) }),
      replace: true,
    });
  // The poll of the relays waits while a confirmation is open.
  const [holdPoll, setHoldPoll] = useState(false);

  const [steps, setSteps] = useState<RadioOfferStep[]>([]);
  const unsaved = useUnsavedChanges(steps.length > 0);

  const config = useQuery(radioConfigQuery);
  const catalog = useQuery(radioCatalogQuery);
  const offer = useQuery(radioOfferQuery);
  const relays = useQuery({
    ...radioRelaysQuery,
    refetchInterval: tab === 'uebersicht' && !holdPoll ? RADIO_RELAYS_POLL_MS : false,
  });

  const listeners = (relays.data?.relays ?? []).reduce(
    (sum, relay) => sum + relay.listeners.length,
    0,
  );
  const state = catalog.data;
  const totals = offer.data?.totals;
  const offerEmpty = totals !== undefined && totals.offered === 0;
  const status = (
    <StatusStrip
      label={t('admin.radio.strip.label')}
      tiles={[
        {
          key: 'catalog',
          label: t('admin.radio.strip.catalog'),
          value: state ? number.format(state.stationsTotal) : '–',
          note: state
            ? t('admin.radio.strip.catalogNote', { count: state.entriesTotal })
            : undefined,
        },
        {
          key: 'offer',
          label: t('admin.radio.strip.offer'),
          value: totals
            ? t('admin.radio.strip.offerValue', {
                stations: number.format(totals.offered),
                countries: number.format(totals.countries),
              })
            : '–',
          note: t('admin.radio.strip.offerNote'),
          tone: offerEmpty ? 'warning' : 'neutral',
        },
        {
          key: 'reachable',
          label: t('admin.radio.strip.reachable'),
          value:
            state && state.entriesTotal > 0
              ? percent.format(state.entriesReachable / state.entriesTotal)
              : '–',
          note: t('admin.radio.strip.reachableNote'),
        },
        {
          key: 'live',
          label: t('admin.radio.strip.live'),
          value: relays.data
            ? t('admin.radio.strip.liveValue', {
                relays: number.format(relays.data.relays.length),
                listeners: number.format(listeners),
              })
            : '–',
          note: t('admin.radio.strip.liveNote'),
          tone: relays.data && relays.data.relays.length > 0 ? 'success' : 'neutral',
        },
      ]}
    />
  );

  return (
    <AdminPage
      title={t('admin.radio.title')}
      help={t('admin.radio.intro')}
      liveSeconds={tab === 'uebersicht' && !holdPoll ? RADIO_RELAYS_POLL_MS / 1000 : undefined}
      titleRef={titleRef}
      status={status}
    >
      {config.data?.bufferMode === 'memory' && (
        <Notice tone="warning" title={t('admin.radio.memory.title')}>
          {t('admin.radio.memory.body')}
        </Notice>
      )}
      {config.isError && <Notice tone="danger" title={problemMessage(t, config.error)} />}
      {offerEmpty && tab !== 'angebot' && (
        <Notice
          tone="warning"
          title={t('admin.radio.empty.title')}
          actions={
            <TextLink href={`${paths.adminRadio}?tab=angebot`}>
              {t('admin.radio.empty.action')}
            </TextLink>
          }
        >
          {t('admin.radio.empty.body')}
        </Notice>
      )}
      <Tabs
        label={t('admin.radio.tabs.label')}
        selectedKey={tab}
        onSelectionChange={selectTab}
        tabs={[
          {
            id: 'uebersicht',
            label: t('admin.radio.tabs.overview'),
            content: (
              <OverviewTab
                catalog={catalog}
                offer={offer}
                relays={relays}
                titleRef={titleRef}
                onConfirmOpenChange={setHoldPoll}
              />
            ),
          },
          {
            id: 'angebot',
            label: t('admin.radio.tabs.offer'),
            content: <OfferTab steps={steps} onStepsChange={setSteps} titleRef={titleRef} />,
          },
          {
            id: 'sender',
            label: t('admin.radio.tabs.stations'),
            content: <StationsTab onEdit={openEditor} titleRef={titleRef} />,
          },
          {
            id: 'eigene',
            label: t('admin.radio.tabs.own'),
            content: <OwnStationsTab onEdit={openEditor} titleRef={titleRef} />,
          },
          {
            id: 'einstellungen',
            label: t('admin.radio.tabs.settings'),
            content: <SettingsTab config={config} titleRef={titleRef} />,
          },
        ]}
      />
      {editing !== undefined && Number.isInteger(editing) && (
        <StationEditor stationId={editing} onClose={() => openEditor(undefined)} />
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
