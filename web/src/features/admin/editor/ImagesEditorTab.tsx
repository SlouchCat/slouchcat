import type {
  AdminItem,
  AdminItemImage,
  ImageCandidate,
  ImageKind,
  components,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useId, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import {
  ChoiceChips,
  Chip,
  ImageCandidateGrid,
  Notice,
  SelectField,
  Skeleton,
  Text,
  TextLink,
  type ImageCandidateTile,
} from '@/ui';
import { AriaButton, ImageTile } from '@/ui/aria';
import { candidateDetail, languageLabel, providerLabel } from '../item/ImagesTab';
import {
  candidateFacets,
  filterCandidates,
  MAX_UPLOAD_BYTES,
  UPLOAD_TYPES,
  uploadErrorKey,
  type CandidateFilter,
} from './editorModel';
import styles from './MetadataEditor.module.css';

type ItemImageUpload = components['schemas']['ItemImageUpload'];

/** Tiles shown at once; «Mehr» adds the next ones (as the administration page). */
const PAGE = 40;

/** Every provider or language in the filters. */
const ALL = '*';

/** The name of a kind; the backdrop of an episode is its still. */
const kindName = (t: TFunction, item: AdminItem, kind: ImageKind) =>
  item.kind === 'episode' && kind === 'backdrop'
    ? t('admin.editor.images.kindName.still')
    : t(`admin.metadata.images.kind.${kind}`);

/** «gewählt · gesperrt», «automatisch · TMDB» or «fehlt». */
function stateOf(t: TFunction, image: AdminItemImage | undefined): string {
  if (image?.locked) return t('admin.editor.images.state.locked');
  if (image?.provider)
    return t('admin.editor.images.state.auto', { provider: providerLabel(t, image.provider) });
  return t('admin.editor.images.state.missing');
}

const candidatesKey = (id: string, kind: ImageKind, refresh: boolean) =>
  ['admin', 'image-candidates', id, kind, refresh] as const;

/**
 * The tab «Bilder» of the metadata editor (concept 15 7.4): the kinds of the title with their
 * state, the current image of the chosen kind with «Eigenes Bild hochladen» and «Zurück auf
 * automatisch», then the providers' candidates with a filter by provider and language. A click on
 * a candidate chooses it, applies it at once and locks the kind.
 */
export function ImagesEditorTab({
  item,
  onChanged,
}: {
  item: AdminItem;
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t } = useTranslation();
  const kinds = item.imageKinds ?? [];
  const [chosenKind, setKind] = useState<ImageKind | null>(null);
  const kind = chosenKind && kinds.includes(chosenKind) ? chosenKind : kinds[0];

  if (kind === undefined) {
    return item.kind === 'episode' ? (
      <Notice
        actions={
          <TextLink href={paths.adminMetadata}>{t('admin.editor.images.stillsOffLink')}</TextLink>
        }
      >
        {t('admin.editor.images.stillsOff')}
      </Notice>
    ) : (
      <Notice>{t('admin.editor.images.noKinds')}</Notice>
    );
  }
  const imageOf = (k: ImageKind) => item.images?.find((image) => image.kind === k);
  return (
    <div className={styles.stack}>
      <ChoiceChips
        label={t('admin.editor.images.kinds')}
        value={kind}
        onChange={(value) => setKind(kinds.find((k) => k === value) ?? null)}
        options={kinds.map((k) => ({
          value: k,
          label: t('admin.editor.images.kindState', {
            kind: kindName(t, item, k),
            state: stateOf(t, imageOf(k)),
          }),
        }))}
      />
      {/* A kind of its own per key: the filter, the page and a failed upload start afresh. */}
      <KindPanel
        key={kind}
        item={item}
        kind={kind}
        image={imageOf(kind)}
        name={kindName(t, item, kind)}
        onChanged={onChanged}
      />
    </div>
  );
}

function KindPanel({
  item,
  kind,
  image,
  name,
  onChanged,
}: {
  item: AdminItem;
  kind: ImageKind;
  image: AdminItemImage | undefined;
  name: string;
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const file = useRef<HTMLInputElement>(null);
  const [refresh, setRefresh] = useState(false);
  const [shown, setShown] = useState(PAGE);
  const [filter, setFilter] = useState<CandidateFilter>({ provider: null, lang: null });
  const [tooLarge, setTooLarge] = useState(false);
  const currentId = useId();
  const candidatesId = useId();

  const candidates = useQuery({
    queryKey: candidatesKey(item.id, kind, refresh),
    queryFn: ({ signal }) =>
      request(
        api.GET('/admin/items/{id}/image-candidates', {
          params: { path: { id: item.id }, query: { kind, ...(refresh ? { refresh: true } : {}) } },
          signal,
        }),
      ),
  });
  // A new image of the kind: the candidates mark another one as current, and the message of an
  // earlier failed upload goes.
  const changed = (next: AdminItem, message: string) => {
    void queryClient.invalidateQueries({ queryKey: ['admin', 'image-candidates', item.id, kind] });
    setTooLarge(false);
    upload.reset();
    onChanged(next, message);
  };
  const choose = useMutation({
    mutationFn: (c: ImageCandidate): Promise<AdminItem> =>
      request(
        api.PUT('/admin/items/{id}/images/{kind}', {
          params: { path: { id: item.id, kind } },
          body: { provider: c.provider, source: c.source },
        }),
      ),
    onSuccess: (next) => changed(next, t('admin.item.images.chosenSaved', { kind: name })),
  });
  const clear = useMutation({
    mutationFn: (): Promise<AdminItem> =>
      request(
        api.DELETE('/admin/items/{id}/images/{kind}', { params: { path: { id: item.id, kind } } }),
      ),
    onSuccess: (next) => changed(next, t('admin.item.images.unlocked', { kind: name })),
  });
  const upload = useMutation({
    mutationFn: (picked: File): Promise<AdminItem> => {
      // openapi-fetch sends a FormData as it is (with the CSRF header of every call); its type
      // knows the part only as a string.
      const form = new FormData();
      form.append('file', picked);
      return request(
        api.PUT('/admin/items/{id}/images/{kind}/upload', {
          params: { path: { id: item.id, kind } },
          body: form as unknown as ItemImageUpload,
        }),
      );
    },
    onSuccess: (next) => changed(next, t('admin.editor.images.upload.done', { kind: name })),
  });
  const onFile = (picked: File | undefined) => {
    if (!picked) return;
    // The server's limit, answered at once instead of after sending 20 MB.
    setTooLarge(picked.size > MAX_UPLOAD_BYTES);
    if (picked.size > MAX_UPLOAD_BYTES) return;
    upload.mutate(picked);
  };
  const uploadError = tooLarge
    ? t('admin.editor.images.upload.error.tooLarge')
    : upload.isError
      ? (() => {
          const key = uploadErrorKey(upload.error);
          return key ? t(key) : problemMessage(t, upload.error);
        })()
      : null;

  const list = candidates.data?.candidates ?? [];
  const facets = candidateFacets(list);
  const filtered = filterCandidates(list, filter);
  const tiles: ImageCandidateTile[] = filtered.slice(0, shown).map((c) => {
    const lang = languageLabel(t, i18n.language, c.lang);
    const detail = candidateDetail(t, i18n.language, c);
    return {
      id: `${c.provider}|${c.source}`,
      previewUrl: c.previewUrl,
      label: lang,
      detail,
      current: c.chosen,
      actionLabel: t('admin.item.images.chooseOne', { label: lang, detail }),
    };
  });
  const current = image?.provider
    ? [
        providerLabel(t, image.provider),
        image.lang ? languageLabel(t, i18n.language, image.lang) : '',
      ]
        .filter(Boolean)
        .join(t('format.separator'))
    : image?.locked
      ? t('admin.editor.images.own')
      : undefined;
  const busy = choose.isPending || clear.isPending || upload.isPending;
  const shape = kind === 'poster' ? 'portrait' : 'landscape';

  return (
    <>
      <section className={styles.current} aria-labelledby={currentId}>
        <div className={styles.preview} data-shape={shape}>
          <ImageTile src={image?.previewUrl} title={item.title} shape={shape} />
        </div>
        <div className={styles.stack}>
          <h3 id={currentId} className={styles.sectionTitle}>
            {current
              ? t('admin.editor.images.current', { source: current })
              : t('admin.editor.images.currentNone')}
          </h3>
          <span>
            <Chip tone={image?.locked ? 'warning' : 'outline'}>{stateOf(t, image)}</Chip>
          </span>
          <div className={styles.actions}>
            <AriaButton
              variant="primary"
              icon="plus"
              isPending={upload.isPending}
              isDisabled={busy}
              onPress={() => file.current?.click()}
            >
              {t('admin.editor.images.upload.action')}
            </AriaButton>
            <input
              ref={file}
              type="file"
              accept={UPLOAD_TYPES}
              hidden
              aria-label={t('admin.editor.images.upload.action')}
              onChange={(event) => {
                onFile(event.currentTarget.files?.[0]);
                // The same file again is a new choice.
                event.currentTarget.value = '';
              }}
            />
            {image?.locked && (
              <AriaButton
                isPending={clear.isPending}
                isDisabled={busy}
                onPress={() => clear.mutate()}
              >
                {t('admin.editor.images.automatic')}
              </AriaButton>
            )}
            <AriaButton
              variant="ghost"
              icon="refresh"
              isDisabled={candidates.isFetching}
              onPress={() => setRefresh(true)}
            >
              {t('admin.item.images.reload')}
            </AriaButton>
          </div>
          <Text tone="secondary" variant="caption">
            {t('admin.editor.images.upload.hint')}
          </Text>
        </div>
      </section>
      {uploadError && (
        <Notice tone="danger" title={t('admin.editor.images.upload.failed')}>
          {uploadError}
        </Notice>
      )}
      {clear.isError && <Notice tone="danger">{problemMessage(t, clear.error)}</Notice>}
      {choose.isError && <Notice tone="danger">{problemMessage(t, choose.error)}</Notice>}

      <section className={styles.stack} aria-labelledby={candidatesId}>
        <h3 id={candidatesId} className={styles.sectionTitle}>
          {t('admin.editor.images.candidates')}
        </h3>
        {candidates.isPending && <Skeleton shape="block" />}
        {candidates.isError && <Notice tone="danger">{problemMessage(t, candidates.error)}</Notice>}
        {candidates.data && candidates.data.unavailable.length > 0 && (
          <Notice tone="warning">
            {t('admin.item.images.unavailable', {
              providers: candidates.data.unavailable
                .map((p) => providerLabel(t, p))
                .join(t('format.listSeparator')),
            })}
          </Notice>
        )}
        {candidates.data && (
          <>
            <div className={styles.actions}>
              <SelectField
                label={t('admin.editor.images.filter.provider')}
                value={filter.provider ?? ALL}
                onChange={(value) => {
                  setFilter({ ...filter, provider: value === ALL ? null : value });
                  setShown(PAGE);
                }}
                options={[
                  { value: ALL, label: t('admin.editor.images.filter.all') },
                  ...facets.providers.map((p) => ({ value: p, label: providerLabel(t, p) })),
                ]}
              />
              <SelectField
                label={t('admin.editor.images.filter.lang')}
                value={filter.lang ?? ALL}
                onChange={(value) => {
                  setFilter({ ...filter, lang: value === ALL ? null : value });
                  setShown(PAGE);
                }}
                options={[
                  { value: ALL, label: t('admin.editor.images.filter.all') },
                  ...facets.langs.map((l) => ({
                    value: l,
                    label: languageLabel(t, i18n.language, l),
                  })),
                ]}
              />
              <Text tone="secondary" variant="caption">
                {t('admin.editor.images.count', { count: filtered.length })}
              </Text>
            </div>
            <Text tone="secondary" variant="caption">
              {t('admin.editor.images.chooseHint')}
            </Text>
            <ImageCandidateGrid
              label={t('admin.item.images.candidates', { kind: name })}
              items={tiles}
              shape={shape}
              emptyText={t('admin.item.images.empty')}
              currentText={t('admin.item.images.current')}
              onAction={(id) => {
                const c = filtered.find((entry) => `${entry.provider}|${entry.source}` === id);
                if (c && !busy) choose.mutate(c);
              }}
            />
            {filtered.length > shown && (
              <div className={styles.actions}>
                <AriaButton onPress={() => setShown((n) => n + PAGE)}>
                  {t('admin.item.images.more', { count: Math.min(PAGE, filtered.length - shown) })}
                </AriaButton>
              </div>
            )}
          </>
        )}
      </section>
    </>
  );
}
