import type { AdminItem } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { Notice, Text } from '@/ui';
import { AriaButton, CheckboxGroup } from '@/ui/aria';
import { MatchSearch } from '../MatchDialog';
import styles from './MetadataEditor.module.css';

/** The tab «Zuordnen»: the search of MatchDialog for films and series. */
export function MatchTab({
  item,
  onChanged,
}: {
  item: AdminItem;
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t } = useTranslation();
  if (item.kind !== 'movie' && item.kind !== 'series') {
    return <Notice>{t('admin.editor.match.notMatchable')}</Notice>;
  }
  return (
    <div className={styles.stack}>
      <Text tone="secondary">{t('admin.editor.match.intro')}</Text>
      <MatchSearch
        itemId={item.id}
        title={item.title}
        year={item.year}
        kind={item.kind}
        onMatched={(next) => onChanged(next, t('admin.maintenance.done.match'))}
      />
    </div>
  );
}

const UNLOCK_ALL = 'unlockAll';

/**
 * The tab «Aktualisieren» (concept 15 7.5): «Jetzt aktualisieren» queues the refresh of every free
 * field and automatically chosen image; the box lifts every lock of the title first. A book reads
 * its files again instead (concept 19 D5.2, «Aus den Dateien neu lesen»).
 */
export function RefreshTab({
  item,
  onChanged,
}: {
  item: AdminItem;
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t } = useTranslation();
  const [options, setOptions] = useState<string[]>([]);
  const book = item.kind === 'audiobook';
  const refresh = useMutation({
    mutationFn: (unlockAll: boolean): Promise<AdminItem> =>
      request(
        api.POST('/admin/items/{id}/refresh', {
          params: { path: { id: item.id } },
          body: { unlockAll },
        }),
      ),
    onSuccess: (next) => {
      setOptions([]);
      onChanged(next, t('admin.item.refreshed'));
    },
  });
  return (
    <div className={styles.stack}>
      <Text tone="secondary">
        {book ? t('admin.editor.refresh.book.body') : t('admin.editor.refresh.body')}
      </Text>
      <CheckboxGroup
        label={t('admin.editor.refresh.options')}
        value={options}
        onChange={setOptions}
        options={[
          {
            value: UNLOCK_ALL,
            label: book
              ? t('admin.editor.refresh.book.unlockAll')
              : t('admin.editor.refresh.unlockAll'),
            description: book
              ? t('admin.editor.refresh.book.unlockAllHint')
              : t('admin.editor.refresh.unlockAllHint'),
          },
        ]}
      />
      <div className={styles.actions}>
        <AriaButton
          variant="primary"
          icon="refresh"
          isPending={refresh.isPending}
          onPress={() => refresh.mutate(options.includes(UNLOCK_ALL))}
        >
          {book ? t('admin.editor.refresh.book.run') : t('admin.editor.refresh.run')}
        </AriaButton>
        <Text tone="secondary" variant="caption">
          {t('admin.editor.refresh.hint')}
        </Text>
      </div>
      {refresh.isError && <Notice tone="danger">{problemMessage(t, refresh.error)}</Notice>}
    </div>
  );
}
