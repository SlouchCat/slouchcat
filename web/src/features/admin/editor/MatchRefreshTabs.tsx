import type { AdminItem } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { firstErrorMessage, problemMessage } from '@/api/errors';
import { Notice, Text } from '@/ui';
import { AriaButton, CheckboxGroup, SettingRow } from '@/ui/aria';
import { providerName } from '../item/facts';
import { MatchSearch } from '../MatchDialog';
import { BookMatchTab, bookMatchSource } from './BookMatchTab';
import styles from './MetadataEditor.module.css';

/** The tab «Zuordnen»: the search of MatchDialog for films and series, a book source for a book. */
export function MatchTab({
  item,
  onChanged,
}: {
  item: AdminItem;
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t } = useTranslation();
  if (item.kind === 'audiobook') return <BookMatchTab item={item} onChanged={onChanged} />;
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
 * its files again («Aus den Dateien neu lesen») or, with a match, fetches the entry of its source
 * again («Vom Anbieter neu holen», concept 19 D5.2); the box applies to both.
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
  const matched = book ? bookMatchSource(item) : undefined;
  const refresh = useMutation({
    mutationFn: ({
      unlockAll,
      fromProvider,
    }: {
      unlockAll: boolean;
      fromProvider?: boolean;
    }): Promise<AdminItem> =>
      request(
        api.POST('/admin/items/{id}/refresh', {
          params: { path: { id: item.id } },
          body: fromProvider ? { unlockAll, fromProvider } : { unlockAll },
        }),
      ),
    onSuccess: (next, { fromProvider }) => {
      setOptions([]);
      onChanged(
        next,
        fromProvider && matched
          ? t('admin.editor.refresh.book.refetched', { source: providerName(t, matched) })
          : t('admin.item.refreshed'),
      );
    },
  });
  const unlockAll = options.includes(UNLOCK_ALL);
  if (book) {
    return (
      <div className={styles.stack}>
        <Text tone="secondary">{t('admin.editor.refresh.book.body')}</Text>
        <div className={styles.refreshWays}>
          <SettingRow
            label={t('admin.editor.refresh.book.filesTitle')}
            help={t('admin.editor.refresh.book.filesHint')}
          >
            <AriaButton
              icon="refresh"
              isPending={refresh.isPending && !refresh.variables?.fromProvider}
              // One request at a time: the other way waits while this one runs.
              isDisabled={refresh.isPending && refresh.variables?.fromProvider === true}
              onPress={() => refresh.mutate({ unlockAll })}
            >
              {t('admin.editor.refresh.book.run')}
            </AriaButton>
          </SettingRow>
          <SettingRow
            label={t('admin.editor.refresh.book.providerTitle')}
            help={
              matched
                ? t('admin.editor.refresh.book.providerHint', {
                    source: providerName(t, matched),
                  })
                : t('admin.editor.refresh.book.providerNone')
            }
          >
            <AriaButton
              icon="refresh"
              isDisabled={!matched || (refresh.isPending && !refresh.variables?.fromProvider)}
              isPending={refresh.isPending && refresh.variables?.fromProvider === true}
              onPress={() => refresh.mutate({ unlockAll, fromProvider: true })}
            >
              {t('admin.editor.refresh.book.providerRun')}
            </AriaButton>
          </SettingRow>
        </div>
        <CheckboxGroup
          label={t('admin.editor.refresh.options')}
          value={options}
          onChange={setOptions}
          options={[
            {
              value: UNLOCK_ALL,
              label: t('admin.editor.refresh.book.unlockAll'),
              description: t('admin.editor.refresh.book.unlockAllHint'),
            },
          ]}
        />
        <Text tone="secondary" variant="caption">
          {t('admin.editor.refresh.hint')}
        </Text>
        {refresh.isError && (
          // `fromProvider` `not_matched` names its field.
          <Notice tone="danger">{firstErrorMessage(t, refresh.error)}</Notice>
        )}
      </div>
    );
  }
  return (
    <div className={styles.stack}>
      <Text tone="secondary">{t('admin.editor.refresh.body')}</Text>
      <CheckboxGroup
        label={t('admin.editor.refresh.options')}
        value={options}
        onChange={setOptions}
        options={[
          {
            value: UNLOCK_ALL,
            label: t('admin.editor.refresh.unlockAll'),
            description: t('admin.editor.refresh.unlockAllHint'),
          },
        ]}
      />
      <div className={styles.actions}>
        <AriaButton
          variant="primary"
          icon="refresh"
          isPending={refresh.isPending}
          onPress={() => refresh.mutate({ unlockAll })}
        >
          {t('admin.editor.refresh.run')}
        </AriaButton>
        <Text tone="secondary" variant="caption">
          {t('admin.editor.refresh.hint')}
        </Text>
      </div>
      {refresh.isError && <Notice tone="danger">{problemMessage(t, refresh.error)}</Notice>}
    </div>
  );
}
