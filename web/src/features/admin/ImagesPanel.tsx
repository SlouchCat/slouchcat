import type { AdminItem, AdminItemImage, ImageCandidate, ImageKind } from '@slouchcat/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import {
  Chip,
  ImageCandidateGrid,
  Notice,
  Panel,
  Skeleton,
  Text,
  type ImageCandidateTile,
} from '@/ui';
import { AriaButton, FormDialog } from '@/ui/aria';
import styles from './AdminItemPage.module.css';

/** Tiles shown at once; "Mehr" adds the next ones (7.3). */
const PAGE = 40;

/** The kinds the administrator chooses per item kind (as the server's choosableKinds). */
const choosable: Partial<Record<AdminItem['kind'], ImageKind[]>> = {
  movie: ['poster', 'backdrop', 'logo'],
  series: ['poster', 'backdrop', 'logo'],
  season: ['poster'],
};

function languageLabel(t: TFunction, language: string, lang: string): string {
  if (!lang) return t('admin.metadata.images.noText');
  try {
    return new Intl.DisplayNames([language], { type: 'language' }).of(lang) ?? lang;
  } catch {
    return lang;
  }
}

function providerLabel(t: TFunction, provider: string): string {
  return t(`admin.item.images.provider.${provider}`, { defaultValue: provider.toUpperCase() });
}

function candidateDetail(t: TFunction, language: string, c: ImageCandidate): string {
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
  return parts.join(' · ');
}

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
      title={t('admin.item.images.pickTitle', { kind: t(`admin.metadata.images.kind.${kind}`) })}
      initialFocus='[role="option"]'
    >
      <div className={styles.layout}>
        {candidates.isPending && <Skeleton shape="block" />}
        {candidates.isError && <Notice tone="danger">{problemMessage(t, candidates.error)}</Notice>}
        {candidates.data && candidates.data.unavailable.length > 0 && (
          <Notice tone="warning">
            {t('admin.item.images.unavailable', {
              providers: candidates.data.unavailable.map((p) => providerLabel(t, p)).join(', '),
            })}
          </Notice>
        )}
        {candidates.data && (
          <ImageCandidateGrid
            label={t('admin.item.images.candidates', {
              kind: t(`admin.metadata.images.kind.${kind}`),
            })}
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

/** One kind: the decided image with source, language, lock; "Andere wählen", "Sperre aufheben". */
function KindRow({
  item,
  kind,
  image,
  onPick,
  onCleared,
}: {
  item: AdminItem;
  kind: ImageKind;
  image?: AdminItemImage;
  onPick: () => void;
  onCleared: (next: AdminItem) => void;
}) {
  const { t, i18n } = useTranslation();
  const clear = useMutation({
    mutationFn: (): Promise<AdminItem> =>
      request(
        api.DELETE('/admin/items/{id}/images/{kind}', { params: { path: { id: item.id, kind } } }),
      ),
    onSuccess: onCleared,
  });
  const kindName = t(`admin.metadata.images.kind.${kind}`);
  const labelId = `admin-item-image-${kind}`;
  return (
    <li className={`${styles.row} ${styles.imageRow}`}>
      <span className={styles.imageThumb} data-shape={kind === 'poster' ? 'portrait' : 'landscape'}>
        {image?.previewUrl && (
          <img src={image.previewUrl} alt="" loading="lazy" referrerPolicy="no-referrer" />
        )}
      </span>
      <div className={styles.rowMain}>
        <span id={labelId} className={styles.rowHead}>
          <strong>{kindName}</strong>
          {image?.provider && <Chip tone="outline">{providerLabel(t, image.provider)}</Chip>}
          {image?.provider && (
            <span className={styles.muted}>
              {languageLabel(t, i18n.language, image.lang ?? '')}
            </span>
          )}
        </span>
        <span className={styles.rowHead}>
          {!image && <span className={styles.muted}>{t('admin.item.images.none')}</span>}
          {image?.chosen && <Chip tone="info">{t('admin.item.images.chosen')}</Chip>}
          {image?.locked && !image.provider && (
            <Chip tone="warning">{t('admin.item.images.lockedNoChoice')}</Chip>
          )}
          {image?.locked && image.provider && (
            <Chip tone="outline">{t('admin.item.images.locked')}</Chip>
          )}
        </span>
        {clear.isError && <Notice tone="danger">{problemMessage(t, clear.error)}</Notice>}
      </div>
      <div className={styles.actions}>
        <AriaButton aria-describedby={labelId} onPress={onPick}>
          {t('admin.item.images.pick')}
        </AriaButton>
        {image?.locked && (
          <AriaButton
            variant="ghost"
            aria-describedby={labelId}
            isPending={clear.isPending}
            onPress={() => clear.mutate()}
          >
            {t('admin.item.images.unlock')}
          </AriaButton>
        )}
      </div>
    </li>
  );
}

/**
 * The section "Bilder" of the title page (concept 10 7.3): per kind the decided image with source,
 * language and lock; "Andere wählen" opens the candidates, a choice locks the kind, "Sperre
 * aufheben" lets the automatic choice run again.
 */
export function ImagesPanel({
  item,
  onChanged,
}: {
  item: AdminItem;
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t } = useTranslation();
  const kinds = choosable[item.kind];
  const [picking, setPicking] = useState<ImageKind | null>(null);
  const opener = useRef<HTMLElement | null>(null);
  if (!kinds) return null;
  return (
    <Panel as="section" aria-labelledby="admin-item-images">
      <h2 id="admin-item-images" className={styles.panelTitle}>
        {t('admin.item.images.title')}
      </h2>
      <Text tone="secondary">{t('admin.item.images.intro')}</Text>
      <ul className={styles.list} aria-label={t('admin.item.images.title')}>
        {kinds.map((kind) => (
          <KindRow
            key={kind}
            item={item}
            kind={kind}
            image={item.images?.find((img) => img.kind === kind)}
            onPick={() => {
              opener.current = document.activeElement as HTMLElement | null;
              setPicking(kind);
            }}
            onCleared={(next) =>
              onChanged(
                next,
                t('admin.item.images.unlocked', { kind: t(`admin.metadata.images.kind.${kind}`) }),
              )
            }
          />
        ))}
      </ul>
      {picking && (
        <PickerDialog
          item={item}
          kind={picking}
          onClose={() => setPicking(null)}
          onChosen={(next) =>
            onChanged(
              next,
              t('admin.item.images.chosenSaved', {
                kind: t(`admin.metadata.images.kind.${picking}`),
              }),
            )
          }
        />
      )}
    </Panel>
  );
}
