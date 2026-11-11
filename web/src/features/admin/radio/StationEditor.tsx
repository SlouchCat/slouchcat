import type { components } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { stationPlace } from '@/app/radio/labels';
import { formatBitrate } from '@/lib/format';
import { Notice, Skeleton, StationLogo, Text } from '@/ui';
import {
  AriaButton,
  EditorDialog,
  Form,
  LockToggle,
  RadioGroup,
  StatusRegion,
  Switch,
  TextField,
  useStatusMessage,
} from '@/ui/aria';
import { MAX_UPLOAD_BYTES, UPLOAD_TYPES } from '../editor/editorModel';
import {
  adminRadioStationQuery,
  deleteRadioLogo,
  probeRadioStation,
  radioAdminKeys,
  updateAdminRadioStation,
  uploadRadioLogo,
  type AdminRadioStation,
  type AdminRadioStationPatch,
} from '../queries';
import { languageCodes } from './languages';
import styles from './Radio.module.css';

type LockField = components['schemas']['RadioLockField'];

const AUTO_STREAM = 'auto';

interface Draft {
  name: string;
  region: string;
  /** A stream's id, or AUTO_STREAM for the ranking. */
  stream: string;
  recommended: boolean;
  locks: LockField[];
  /** Own stations only (contract 1.19.0): the stream address, homepage, country, languages. */
  streamUrl: string;
  homepage: string;
  country: string;
  languages: string;
}

const preferredOf = (station: AdminRadioStation) =>
  String(station.streams.find((stream) => stream.preferred)?.id ?? AUTO_STREAM);

/** The stream address of an own station: its one stream, the preferred if there are more. */
const ownStreamUrl = (station: AdminRadioStation) =>
  (station.streams.find((stream) => stream.preferred) ?? station.streams[0])?.url ?? '';

const draftOf = (station: AdminRadioStation): Draft => ({
  name: station.name,
  region: station.region ?? '',
  stream: preferredOf(station),
  recommended: station.recommended,
  locks: station.locks,
  streamUrl: ownStreamUrl(station),
  homepage: station.homepage ?? '',
  country: station.country ?? '',
  languages: station.languages.join(', '),
});

const sameSet = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && a.every((value) => b.includes(value));

/**
 * The change for the server: the fields that differ, and the locks. A field that is set locks
 * itself (S2), so its lock joins the set; `locks` replaces the whole set on the server.
 */
function patchOf(station: AdminRadioStation, draft: Draft): AdminRadioStationPatch {
  const patch: AdminRadioStationPatch = {};
  const locks = new Set(draft.locks);
  const name = draft.name.trim();
  if (name !== '' && name !== station.name) {
    patch.name = name;
    locks.add('name');
  }
  const region = draft.region.trim().toUpperCase();
  if (region !== (station.region ?? '')) {
    patch.region = region === '' ? null : region;
    locks.add('region');
  }
  if (draft.stream !== preferredOf(station)) {
    patch.preferredStreamId = draft.stream === AUTO_STREAM ? null : Number(draft.stream);
    if (draft.stream !== AUTO_STREAM) locks.add('stream');
    else locks.delete('stream');
  }
  if (station.source === 'own') {
    // The sync never touches an own station: these fields carry no lock.
    const streamUrl = draft.streamUrl.trim();
    if (streamUrl !== '' && streamUrl !== ownStreamUrl(station)) patch.streamUrl = streamUrl;
    const homepage = draft.homepage.trim();
    if (homepage !== (station.homepage ?? '')) patch.homepage = homepage === '' ? null : homepage;
    const country = draft.country.trim().toUpperCase();
    if (country !== (station.country ?? '')) patch.country = country === '' ? null : country;
    const languages = languageCodes(draft.languages);
    if (!sameSet(languages, station.languages)) patch.languages = languages;
  }
  if (draft.recommended !== station.recommended) patch.recommended = draft.recommended;
  if (!sameSet([...locks], station.locks)) patch.locks = [...locks];
  return patch;
}

interface StationEditorProps {
  stationId: number;
  onClose: () => void;
}

/**
 * The editor of one station (concept 16, 5.5; `?item=<id>`): name, logo, region and stream, each
 * with its lock as in the metadata editor (concept 15), the stream check and «Empfohlen». Genre,
 * kind and mood come from the rules file only since contract 1.28.0 (concept 19 part B, B5).
 */
export function StationEditor({ stationId, onClose }: StationEditorProps) {
  const { t, i18n } = useTranslation();
  const station = useQuery(adminRadioStationQuery(stationId));
  const data = station.data;
  return (
    <EditorDialog
      onClose={onClose}
      title={data?.name ?? t('common.loading')}
      subtitle={data ? stationPlace(t, i18n.language, data) : undefined}
      imageUrl={imageUrl(data?.logo)}
      imageShape="portrait"
      closeLabel={t('admin.radio.editor.close')}
    >
      {station.isError ? (
        <Notice tone="danger" title={problemMessage(t, station.error)} />
      ) : data ? (
        <EditorForm key={data.updatedAt} station={data} />
      ) : (
        <Skeleton shape="block" />
      )}
    </EditorDialog>
  );
}

function EditorForm({ station }: { station: AdminRadioStation }) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const [draft, setDraft] = useState(() => draftOf(station));
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) =>
    setDraft((previous) => ({ ...previous, [key]: value }));
  const patch = patchOf(station, draft);
  const dirty = Object.keys(patch).length > 0;
  const file = useRef<HTMLInputElement>(null);
  const [tooLarge, setTooLarge] = useState(false);

  const changed = async (next: AdminRadioStation, text: string) => {
    queryClient.setQueryData(adminRadioStationQuery(station.id).queryKey, next);
    setMessage(text);
    await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
  };
  const save = useMutation({
    mutationFn: () => updateAdminRadioStation(station.id, patch),
    onSuccess: (next) => changed(next, t('admin.radio.editor.saved', { name: next.name })),
  });
  const upload = useMutation({
    mutationFn: (picked: File) => uploadRadioLogo(station.id, picked),
    onSuccess: (next) => changed(next, t('admin.radio.editor.saved', { name: next.name })),
  });
  const resetLogo = useMutation({
    mutationFn: () => deleteRadioLogo(station.id),
    onSuccess: async () => {
      setMessage(t('admin.radio.editor.saved', { name: station.name }));
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
    },
  });
  const probe = useMutation({
    mutationFn: () =>
      probeRadioStation(
        station.id,
        draft.stream === AUTO_STREAM ? undefined : Number(draft.stream),
      ),
  });
  const error = save.error ?? upload.error ?? resetLogo.error;
  // A refused stream address or homepage (`local_address`, F-15) stands at its field.
  const saveErrors = save.error ? formErrors(t, save.error) : null;

  const isLocked = (field: LockField) => draft.locks.includes(field);
  const lock = (field: LockField, fieldLabel: string) => (
    <LockToggle
      label={t('admin.radio.editor.lock', { field: fieldLabel })}
      isLocked={isLocked(field)}
      lockedText={t('admin.radio.editor.locked')}
      freeText={t('admin.radio.editor.free')}
      onChange={() =>
        set(
          'locks',
          isLocked(field)
            ? draft.locks.filter((entry) => entry !== field)
            : [...draft.locks, field],
        )
      }
    />
  );
  const field = (control: ReactNode, lockField: LockField, label: string) => (
    <div className={styles.field}>
      <div className={styles.fieldControl}>{control}</div>
      {lock(lockField, label)}
    </div>
  );

  const result = probe.data;
  return (
    <div className={styles.editor}>
      <StatusRegion message={message} />
      {error && <Notice tone="danger">{problemMessage(t, error)}</Notice>}
      <p className={styles.muted}>{t('admin.radio.editor.lockHelp')}</p>

      {field(
        <TextField
          label={t('admin.radio.editor.name')}
          value={draft.name}
          onChange={(value) => set('name', value)}
        />,
        'name',
        t('admin.radio.editor.name'),
      )}

      {station.source === 'own' && (
        <Form
          className={styles.editor}
          validationErrors={saveErrors?.fields}
          onSubmit={(event) => {
            event.preventDefault();
            if (dirty) save.mutate();
          }}
        >
          <p className={styles.muted}>{t('admin.radio.own.editorHelp')}</p>
          <TextField
            name="streamUrl"
            type="url"
            label={t('admin.radio.own.fields.streamUrl')}
            description={t('admin.radio.own.fields.streamUrlHelp')}
            value={draft.streamUrl}
            onChange={(value) => set('streamUrl', value)}
            autoComplete="off"
            spellCheck="false"
            mono
          />
          <TextField
            name="homepage"
            type="url"
            label={t('admin.radio.own.fields.homepage')}
            description={t('admin.radio.own.fields.homepageHelp')}
            value={draft.homepage}
            onChange={(value) => set('homepage', value)}
            autoComplete="off"
            spellCheck="false"
            mono
          />
          <TextField
            name="country"
            label={t('admin.radio.own.fields.country')}
            description={t('admin.radio.own.fields.countryHelp')}
            value={draft.country}
            onChange={(value) => set('country', value)}
          />
          <TextField
            name="languages"
            label={t('admin.radio.own.fields.languages')}
            description={t('admin.radio.own.fields.languagesHelp')}
            value={draft.languages}
            onChange={(value) => set('languages', value)}
          />
        </Form>
      )}

      <div className={styles.field}>
        <div className={styles.logoRow}>
          <StationLogo name={station.name} src={imageUrl(station.logo)} size="md" />
          <div className={styles.fieldControl}>
            <Text>{t('admin.radio.editor.logo')}</Text>
            <Text tone="secondary">{t(`admin.radio.editor.logoState.${station.logoState}`)}</Text>
            <div className={styles.actions}>
              <AriaButton
                icon="plus"
                isPending={upload.isPending}
                onPress={() => file.current?.click()}
              >
                {t('admin.radio.editor.logoUpload')}
              </AriaButton>
              <input
                ref={file}
                type="file"
                accept={UPLOAD_TYPES}
                hidden
                aria-label={t('admin.radio.editor.logoUpload')}
                onChange={(event) => {
                  const picked = event.currentTarget.files?.[0];
                  // The same file again is a new choice.
                  event.currentTarget.value = '';
                  if (!picked) return;
                  setTooLarge(picked.size > MAX_UPLOAD_BYTES);
                  if (picked.size <= MAX_UPLOAD_BYTES) upload.mutate(picked);
                }}
              />
              {station.logoState === 'own' && (
                <AriaButton isPending={resetLogo.isPending} onPress={() => resetLogo.mutate()}>
                  {t('admin.radio.editor.logoReset')}
                </AriaButton>
              )}
            </div>
            {tooLarge && <Notice tone="danger">{t('admin.radio.editor.logoTooLarge')}</Notice>}
          </div>
        </div>
      </div>

      {field(
        <TextField
          label={t('admin.radio.editor.region')}
          description={t('admin.radio.editor.regionHelp')}
          value={draft.region}
          onChange={(value) => set('region', value)}
        />,
        'region',
        t('admin.radio.editor.region'),
      )}
      {field(
        <RadioGroup
          label={t('admin.radio.editor.stream')}
          orientation="vertical"
          options={[
            { value: AUTO_STREAM, label: t('admin.radio.editor.streamAuto') },
            ...station.streams.map((stream) => ({
              value: String(stream.id),
              label: [
                stream.codec,
                stream.bitrateKbps !== undefined &&
                  formatBitrate(stream.bitrateKbps * 1000, language),
                stream.reachable ? undefined : t('admin.radio.editor.unreachable'),
                stream.urlResolved ?? stream.url,
              ]
                .filter(Boolean)
                .join(' · '),
            })),
          ]}
          value={draft.stream}
          onChange={(value) => set('stream', value)}
        />,
        'stream',
        t('admin.radio.editor.stream'),
      )}
      <div className={styles.actions}>
        <AriaButton icon="activity" isPending={probe.isPending} onPress={() => probe.mutate()}>
          {t('admin.radio.editor.probe')}
        </AriaButton>
      </div>
      {probe.error && <Notice tone="warning">{problemMessage(t, probe.error)}</Notice>}
      {result && (
        <Notice title={t('admin.radio.editor.probe')}>
          <p>
            {t('admin.radio.editor.probeResult', {
              codec: result.codec,
              bitrate: result.bitrateKbps,
              sampleRate: result.sampleRate,
              channels: result.channels,
              ms: result.responseMs,
            })}
          </p>
          <p>
            {result.hasMetadata
              ? t('admin.radio.editor.probeTitles')
              : t('admin.radio.editor.probeNoTitles')}
          </p>
          {result.title && <p>{t('admin.radio.editor.probeTitle', { title: result.title })}</p>}
        </Notice>
      )}

      <Switch isSelected={draft.recommended} onChange={(value) => set('recommended', value)}>
        {t('admin.radio.editor.recommended')}
      </Switch>
      <Text tone="secondary">{t('admin.radio.editor.recommendedHelp')}</Text>

      {station.tags.length > 0 && (
        <p className={styles.muted}>
          {t('admin.radio.editor.tags')}: {station.tags.join(', ')}
        </p>
      )}

      <div className={styles.actions}>
        <AriaButton
          variant="primary"
          isPending={save.isPending}
          onPress={() => {
            if (dirty) save.mutate();
          }}
        >
          {t('admin.radio.editor.save')}
        </AriaButton>
      </div>
    </div>
  );
}
