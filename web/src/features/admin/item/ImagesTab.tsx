import type { AdminItem, AdminItemImage, ImageCandidate, ImageKind } from '@slouchcat/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import {
  Chip,
  ImageCandidateGrid,
  Notice,
  Skeleton,
  Text,
  VisuallyHidden,
  type ImageCandidateTile,
} from '@/ui';
import { AriaButton, DataTable, FormDialog, type DataColumn } from '@/ui/aria';
import styles from './AdminItem.module.css';

/** Tiles shown at once; "Mehr" adds the next ones (concept 10, 7.3). */
const PAGE = 40;

/** The kinds the administrator chooses per item kind (as the server's choosableKinds). */
const choosable: Partial<Record<AdminItem['kind'], ImageKind[]>> = {
  movie: ['poster', 'backdrop', 'logo'],
  series: ['poster', 'backdrop', 'logo'],
  season: ['poster'],
};

/** The image kinds of the tab «Bilder»; undefined: the title has no such tab (an episode). */
export const imageKindsOf = (item: AdminItem): ImageKind[] | undefined => choosable[item.kind];

export function languageLabel(t: TFunction, language: string, lang: string): string {
  if (!lang) return t('admin.metadata.images.noText');
  try {
    return new Intl.DisplayNames([language], { type: 'language' }).of(lang) ?? lang;
  } catch {
    return lang;
  }
}

export function providerLabel(t: TFunction, provider: string): string {
  return t(`admin.item.images.provider.${provider}`, { defaultValue: provider.toUpperCase() });
}

export function candidateDetail(t: TFunction, language: string, c: ImageCandidate): string {
  const parts: string[] = [];
  if (c.width > 0 && c.height > 0)
    parts.push(t('admin.item.images.size', { width: c.width, height: c.height }));
  parts.push(providerLabel(t, c.provider));
  if (c.score > 0)
    parts.push(
      t('admin.item.images.score', {
        score: new Intl.NumberFormat(language, { maximumFractionDigits: 1 }).format(c.score),
      }),
    );
  return parts.join(t('format.separator'));
}

const kindName = (t: TFunction, kind: ImageKind) => t(`admin.metadata.images.kind.${kind}`);

/** The candidates of one kind in a dialog: a grid, "Mehr", Enter chooses and locks. */
function PickerDialog({
  item,
  kind,
  onClose,
  onChosen,
}: {
  item: AdminItem;
  kind: ImageKind;
  onClose: () => void;
  onChosen: (next: AdminItem) => void;
}) {
  const { t, i18n } = useTranslation();
  const [shown, setShown] = useState(PAGE);
  const [refresh, setRefresh] = useState(false);
  const candidates = useQuery({
    queryKey: ['admin', 'image-candidates', item.id, kind, refresh],
    queryFn: ({ signal }) =>
      request(
        api.GET('/admin/items/{id}/image-candidates', {
          params: { path: { id: item.id }, query: { kind, ...(refresh ? { refresh: true } : {}) } },
          signal,
        }),
      ),
  });
  const choose = useMutation({
    mutationFn: (c: ImageCandidate): Promise<AdminItem> =>
      request(
        api.PUT('/admin/items/{id}/images/{kind}', {
          params: { path: { id: item.id, kind } },
          body: { provider: c.provider, source: c.source },
        }),
      ),
    onSuccess: (next) => {
      onChosen(next);
      onClose();
    },
  });
  const list = candidates.data?.candidates ?? [];
  const tiles: ImageCandidateTile[] = list.slice(0, shown).map((c) => {
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
  return (
    <FormDialog
      isOpen
      size="wide"
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.item.images.pickTitle', { kind: kindName(t, kind) })}
      initialFocus='[role="option"]'
    >
      <div className={styles.stack}>
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
          <ImageCandidateGrid
            label={t('admin.item.images.candidates', { kind: kindName(t, kind) })}
            items={tiles}
            shape={kind === 'poster' ? 'portrait' : 'landscape'}
            emptyText={t('admin.item.images.empty')}
            currentText={t('admin.item.images.current')}
            autoFocus
            onAction={(id) => {
              const c = list.find((entry) => `${entry.provider}|${entry.source}` === id);
              if (c && !choose.isPending) choose.mutate(c);
            }}
          />
        )}
        {choose.isError && <Notice tone="danger">{problemMessage(t, choose.error)}</Notice>}
        <div className={styles.actions}>
          {list.length > shown && (
            <AriaButton onPress={() => setShown((n) => n + PAGE)}>
              {t('admin.item.images.more', { count: Math.min(PAGE, list.length - shown) })}
            </AriaButton>
          )}
          <AriaButton
            icon="refresh"
            isDisabled={candidates.isFetching}
            onPress={() => setRefresh(true)}
          >
            {t('admin.item.images.reload')}
          </AriaButton>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
        </div>
      </div>
    </FormDialog>
  );
}

interface KindRow {
  kind: ImageKind;
  image?: AdminItemImage;
}

/**
 * The tab «Bilder» (TIT-5, concept 10 7.3): per kind the decided image with source, language and
 * lock in a table; «Andere wählen» opens the candidates in a dialog, a choice locks the kind,
 * «Sperre aufheben» lets the automatic choice run again.
 */
export function ImagesTab({
  item,
  kinds,
  onChanged,
}: {
  item: AdminItem;
  kinds: readonly ImageKind[];
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const [picking, setPicking] = useState<ImageKind | null>(null);
  const clear = useMutation({
    mutationFn: (kind: ImageKind): Promise<AdminItem> =>
      request(
        api.DELETE('/admin/items/{id}/images/{kind}', { params: { path: { id: item.id, kind } } }),
      ),
    onSuccess: (next, kind) =>
      onChanged(next, t('admin.item.images.unlocked', { kind: kindName(t, kind) })),
  });
  const labelId = (kind: ImageKind) => `admin-item-image-${kind}`;
  const columns: DataColumn<KindRow>[] = [
    {
      key: 'kind',
      header: t('admin.item.images.columns.kind'),
      rowHeader: true,
      cell: ({ kind, image }) => (
        <span className={styles.kindCell}>
          <span className={styles.thumb} data-shape={kind === 'poster' ? 'portrait' : 'landscape'}>
            {image?.previewUrl && (
              <img src={image.previewUrl} alt="" loading="lazy" referrerPolicy="no-referrer" />
            )}
          </span>
          <strong id={labelId(kind)}>{kindName(t, kind)}</strong>
        </span>
      ),
    },
    {
      key: 'source',
      header: t('admin.item.images.columns.source'),
      cell: ({ image }) =>
        image?.provider ? (
          <span className={styles.chips}>
            <Chip tone="outline">{providerLabel(t, image.provider)}</Chip>
            <span className={styles.muted}>
              {languageLabel(t, i18n.language, image.lang ?? '')}
            </span>
          </span>
        ) : (
          <span className={styles.muted}>{t('admin.item.images.none')}</span>
        ),
    },
    {
      key: 'state',
      header: t('admin.item.images.columns.state'),
      cell: ({ image }) => (
        <span className={styles.chips}>
          {image?.chosen && <Chip tone="info">{t('admin.item.images.chosen')}</Chip>}
          {image?.locked && !image.provider && (
            <Chip tone="warning">{t('admin.item.images.lockedNoChoice')}</Chip>
          )}
          {image?.locked && image.provider && (
            <Chip tone="outline">{t('admin.item.images.locked')}</Chip>
          )}
        </span>
      ),
    },
    {
      key: 'actions',
      header: t('admin.item.images.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: ({ kind, image }) => (
        <span className={styles.rowActions}>
          <AriaButton aria-describedby={labelId(kind)} onPress={() => setPicking(kind)}>
            {t('admin.item.images.pick')}
          </AriaButton>
          {image?.locked && (
            <AriaButton
              variant="ghost"
              aria-describedby={labelId(kind)}
              isPending={clear.isPending && clear.variables === kind}
              onPress={() => clear.mutate(kind)}
            >
              {t('admin.item.images.unlock')}
            </AriaButton>
          )}
        </span>
      ),
    },
  ];
  return (
    <section className={styles.tabBody} aria-labelledby="admin-item-images">
      <VisuallyHidden as="h2" id="admin-item-images">
        {t('admin.item.images.title')}
      </VisuallyHidden>
      <Text tone="secondary">{t('admin.item.images.intro')}</Text>
      {clear.isError && clear.variables && (
        <Notice
          tone="danger"
          title={t('admin.item.images.unlockFailed', { kind: kindName(t, clear.variables) })}
        >
          {problemMessage(t, clear.error)}
        </Notice>
      )}
      <DataTable
        caption={t('admin.item.images.title')}
        columns={columns}
        rows={kinds.map((kind) => ({ kind, image: item.images?.find((img) => img.kind === kind) }))}
        rowKey={(row) => row.kind}
      />
      {picking && (
        <PickerDialog
          item={item}
          kind={picking}
          onClose={() => setPicking(null)}
          onChosen={(next) =>
            onChanged(next, t('admin.item.images.chosenSaved', { kind: kindName(t, picking) }))
          }
        />
      )}
    </section>
  );
}
