import type { AdminItem } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { Notice, Skeleton, TextLink, useSpatialHold } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  EditorDialog,
  StatusRegion,
  Tabs,
  useStatusMessage,
} from '@/ui/aria';
import { visibleLibrariesQuery } from '@/features/catalog/queries';
import { DeleteItemDialog } from '../deletion/DeleteItemDialog';
import { adminItemQuery } from '../queries';
import { ImagesEditorTab } from './ImagesEditorTab';
import { emptyDraft, type EditorDraft } from './editorModel';
import { BookMatchNote } from './BookMatchTab';
import { MatchTab, RefreshTab } from './MatchRefreshTabs';
import { MetadataEditorTab } from './MetadataEditorTab';
import styles from './MetadataEditor.module.css';

interface MetadataEditorProps {
  itemId: string;
  /** Escape or «Schliessen»; the holder unmounts the editor and reads the title again. */
  onClose: () => void;
}

/** The small picture of the head: the poster, the still of an episode, the square cover of a book. */
function headImage(item: AdminItem) {
  const kind = item.kind === 'episode' ? 'backdrop' : 'poster';
  return {
    url: item.images?.find((image) => image.kind === kind)?.previewUrl,
    shape:
      item.kind === 'episode'
        ? ('landscape' as const)
        : item.kind === 'audiobook'
          ? ('square' as const)
          : ('portrait' as const),
  };
}

/** The kinds with a delete (concept 20 P-9); a podcast episode only with a file (O-5). */
function deletableInEditor(item: AdminItem): boolean {
  switch (item.kind) {
    case 'movie':
    case 'series':
    case 'season':
    case 'episode':
    case 'audiobook':
    case 'podcast':
      return true;
    case 'podcast_episode':
      return item.mediaFiles.length > 0;
    default:
      return false;
  }
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
  // The pen sits on browsing pages, which hold this list already.
  const libraries = useQuery(visibleLibrariesQuery);
  // Unset until the reader picks a tab: a book starts on «Metadaten», every other title on «Bilder».
  const [picked, setTab] = useState<string | null>(null);
  const [message, setMessage] = useStatusMessage();
  const [draft, setDraft] = useState<EditorDraft>(emptyDraft);
  const [discarding, setDiscarding] = useState(false);
  // «Löschen …» (concept 20 P-9): the way to delete a season, whose pen sits on its cover.
  const [deleting, setDeleting] = useState(false);
  const dirty = draft.unlock.length > 0 || Object.keys(draft.changed).length > 0;
  // Escape or «Schliessen» with unsaved metadata asks first; a clean draft closes at once.
  const requestClose = () => (dirty ? setDiscarding(true) : onClose());

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
  // A book (concept 19 D5.2): «Metadaten», «Cover», «Zuordnen» (a book source), «Aktualisieren».
  const book = data.kind === 'audiobook';
  const tab = picked ?? (book && data.metadata ? 'metadata' : 'images');
  const imagesTab = {
    id: 'images',
    label: book ? t('admin.editor.tabs.cover') : t('admin.editor.tabs.images'),
    content: <ImagesEditorTab item={data} onChanged={adopt} />,
  };
  const metadataTab = data.metadata
    ? [
        {
          id: 'metadata',
          label: t('admin.editor.tabs.metadata'),
          // A dot while there are unsaved changes, so they show from every tab.
          ...(dirty
            ? {
                count: t('admin.item.tabs.unsavedMark'),
                countLabel: t('admin.item.rating.unsaved'),
                tone: 'warning' as const,
              }
            : {}),
          content: (
            <MetadataEditorTab
              item={data}
              meta={data.metadata}
              draft={draft}
              onDraftChange={setDraft}
              onChanged={adopt}
            />
          ),
        },
      ]
    : [];
  const matchTab = {
    id: 'match',
    label: t('admin.editor.tabs.match'),
    content: <MatchTab item={data} onChanged={adopt} />,
  };
  const refreshTab = {
    id: 'refresh',
    label: t('admin.editor.tabs.refresh'),
    content: <RefreshTab item={data} onChanged={adopt} />,
  };
  return (
    <EditorDialog
      // `title` is the item row; an edited title lives in the server language's text.
      title={data.metadata?.title ?? data.title}
      subtitle={
        library
          ? t('admin.editor.subtitle', { kind: t(`kind.${data.kind}`), library })
          : t(`kind.${data.kind}`)
      }
      imageUrl={image.url}
      imageShape={image.shape}
      link={
        <span className={styles.links}>
          <TextLink href={paths.adminItem(data.id)}>{t('admin.editor.openAdmin')}</TextLink>
          {deletableInEditor(data) && (
            <AriaButton variant="ghost" icon="trash" onPress={() => setDeleting(true)}>
              {t('admin.itemDelete.editorAction')}
            </AriaButton>
          )}
        </span>
      }
      closeLabel={t('admin.editor.close')}
      onClose={requestClose}
    >
      {deleting && (
        <DeleteItemDialog
          itemId={data.id}
          isOpen
          onOpenChange={setDeleting}
          // The title is gone: the editor closes, and the page behind reads its list again.
          onDeleted={() => {
            setDeleting(false);
            onClose();
          }}
        />
      )}
      <ConfirmDialog
        isOpen={discarding}
        onOpenChange={setDiscarding}
        title={t('admin.editor.discard.title')}
        body={t('admin.editor.discard.body')}
        confirmLabel={t('admin.editor.discard.confirm')}
        cancelLabel={t('admin.editor.discard.cancel')}
        tone="danger"
        defaultFocus="cancel"
        onConfirm={onClose}
      />
      <div className={styles.body}>
        <StatusRegion message={message} />
        {book && <BookMatchNote item={data} />}
        <Tabs
          label={t('admin.editor.tabs.label')}
          selectedKey={tab}
          onSelectionChange={setTab}
          tabs={
            book
              ? [...metadataTab, imagesTab, matchTab, refreshTab]
              : [imagesTab, ...metadataTab, matchTab, refreshTab]
          }
        />
      </div>
    </EditorDialog>
  );
}
