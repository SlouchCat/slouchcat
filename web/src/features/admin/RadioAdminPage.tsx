import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice } from '@/ui';
import { AdminPage, StatusStrip, Tabs, useSearchSelection } from '@/ui/aria';
import {
  RADIO_RELAYS_POLL_MS,
  radioCatalogQuery,
  radioConfigQuery,
  radioRelaysQuery,
} from './queries';
import { OverviewTab } from './radio/OverviewTab';
import { OwnStationsTab } from './radio/OwnStationsTab';
import { SettingsTab } from './radio/SettingsTab';
import { StationEditor } from './radio/StationEditor';
import { StationsTab } from './radio/StationsTab';

const route = getRouteApi('/_authed/_active/_chrome/admin/radio');

/**
 * The tabs (R-16, concept 16 stage 2); the ids are part of links and stay stable. «Einordnung» is
 * gone with the tag rules (contract 1.28.0, concept 19 part B, B5).
 */
const TABS = ['uebersicht', 'sender', 'eigene', 'einstellungen'] as const;

/**
 * Webradio in the administration (concept 16, 11; variant A «Reiter»): the counters, a warning
 * without RAM buffer, the tabs «Übersicht» (catalog, live relays, server load), «Sender» (the
 * whole catalog, the editor in `?item=`), «Eigene Sender» and «Einstellungen». The tab is `?tab=`
 * (S-14).
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

  const config = useQuery(radioConfigQuery);
  const catalog = useQuery(radioCatalogQuery);
  const relays = useQuery({
    ...radioRelaysQuery,
    refetchInterval: tab === 'uebersicht' && !holdPoll ? RADIO_RELAYS_POLL_MS : false,
  });

  const listeners = (relays.data?.relays ?? []).reduce(
    (sum, relay) => sum + relay.listeners.length,
    0,
  );
  const state = catalog.data;
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
                relays={relays}
                titleRef={titleRef}
                onConfirmOpenChange={setHoldPoll}
              />
            ),
          },
          {
            id: 'sender',
            label: t('admin.radio.tabs.stations'),
            content: <StationsTab onEdit={openEditor} />,
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
    </AdminPage>
  );
}
