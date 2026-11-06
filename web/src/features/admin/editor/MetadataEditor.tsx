import type { AdminItem } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { Notice, Skeleton, TextLink, useSpatialHold } from '@/ui';
import { AriaButton, EditorDialog, StatusRegion, Tabs, useStatusMessage } from '@/ui/aria';
import { adminItemQuery, librariesQuery } from '../queries';
import { ImagesEditorTab } from './ImagesEditorTab';
import { MatchTab, RefreshTab } from './MatchRefreshTabs';
import styles from './MetadataEditor.module.css';

interface MetadataEditorProps {
  itemId: string;
  /** Escape or «Schliessen»; the holder unmounts the editor and reads the title again. */
  onClose: () => void;
}

/** The small picture of the head: the poster, the still of an episode. */
function headImage(item: AdminItem) {
  const kind = item.kind === 'episode' ? 'backdrop' : 'poster';
  return {
    url: item.images?.find((image) => image.kind === kind)?.previewUrl,
    shape: item.kind === 'episode' ? ('landscape' as const) : ('portrait' as const),
  };
}

/**
 * The metadata editor of a title (concept 15 7.4, variant A «Dialog»), opened with the pen on the
 * detail page or a library cover: a head with picture, title, kind, library and the link to the
 * administration page, then the tabs. Every change is a request of its own; the holder reads the
 * title again after closing. Only for the administrator's primary profile; the server answers
 * every request only for that session (x-visibility admin).
 */
export function MetadataEditor({ itemId, onClose }: MetadataEditorProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // The page behind keeps its arrow keys to itself while the dialog is open.
  useSpatialHold(true);
  const item = useQuery(adminItemQuery(itemId));
  const libraries = useQuery(librariesQuery);
  const [tab, setTab] = useState('images');
  const [message, setMessage] = useStatusMessage();

  const data = item.data;
  if (!data) {
    return (
      <EditorDialog
        title={t('item.editMetadata')}
        closeLabel={t('admin.editor.close')}
        onClose={onClose}
      >
        {item.isPending && <Skeleton shape="block" />}
        {item.isError && (
          <Notice
            tone="danger"
            title={
              isApiError(item.error, 404)
                ? t('admin.editor.notFound')
                : problemMessage(t, item.error)
            }
            actions={
              !isApiError(item.error, 404) && (
                <AriaButton icon="refresh" onPress={() => void item.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              )
            }
          />
        )}
      </EditorDialog>
    );
  }

  /** A tab changed the title: the answer is the new state of the dialog and of the page. */
  const adopt = (next: AdminItem, text: string) => {
    queryClient.setQueryData(adminItemQuery(itemId).queryKey, next);
    setMessage(text);
  };

  const library = libraries.data?.find((entry) => entry.id === data.libraryId)?.name;
  const image = headImage(data);
  return (
    <EditorDialog
      title={data.title}
      subtitle={
        library
          ? t('admin.editor.subtitle', { kind: t(`kind.${data.kind}`), library })
          : t(`kind.${data.kind}`)
      }
      imageUrl={image.url}
      imageShape={image.shape}
      link={<TextLink href={paths.adminItem(data.id)}>{t('admin.editor.openAdmin')}</TextLink>}
      closeLabel={t('admin.editor.close')}
      onClose={onClose}
    >
      <div className={styles.body}>
        <StatusRegion message={message} />
        <Tabs
          label={t('admin.editor.tabs.label')}
          selectedKey={tab}
          onSelectionChange={setTab}
          tabs={[
            {
              id: 'images',
              label: t('admin.editor.tabs.images'),
              content: <ImagesEditorTab item={data} onChanged={adopt} />,
            },
            {
              id: 'match',
              label: t('admin.editor.tabs.match'),
              content: <MatchTab item={data} onChanged={adopt} />,
            },
            {
              id: 'refresh',
              label: t('admin.editor.tabs.refresh'),
              content: <RefreshTab item={data} onChanged={adopt} />,
            },
          ]}
        />
      </div>
    </EditorDialog>
  );
}
