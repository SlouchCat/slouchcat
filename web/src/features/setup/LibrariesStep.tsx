import type { Library, LibraryTrickplay } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { FactList, Notice, Skeleton } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { AddLibraryDialog, useLibraryDraft } from '../admin/libraries/AddLibraryDialog';
import { librariesQuery } from '../admin/queries';
import { setupPath, setupProgressQuery, updateSetupProgress } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/** «Vorschau beim Spulen» on, as the default of step 5 says (concept 18: «Im Voraus»). */
const TRICKPLAY_ON: LibraryTrickplay = {
  movies: { mode: 'ahead', keep: true },
  series: { mode: 'ahead', keep: true, scope: 'series' },
};

/** A library as the step lists it: kind, scan state and its folders. */
function LibraryFacts({ library }: { library: Library }) {
  const { t } = useTranslation();
  return (
    <section className={styles.section} aria-label={library.name}>
      <h2 className={styles.sectionTitle}>{library.name}</h2>
      <FactList
        facts={[
          [t('admin.libraries.columns.kind'), t(`library.kind.${library.kind}`)],
          [t('admin.libraries.columns.state'), t(`admin.libraries.state.${library.scan.state}`)],
          ...library.roots.map(
            (root, index) =>
              [t('setup.libraries.folder', { position: index + 1 }), root.path] as const,
          ),
        ]}
      />
    </section>
  );
}

/**
 * Step 6 (concept 22, 3.3): the libraries, at least one (F-1). «Bibliothek hinzufügen» is the
 * dialog of Verwaltung › Bibliotheken with its folder browser and folder cards, the trickplay
 * default of step 5 filled in for video; adding queues the first scan at once.
 */
export function LibrariesStep() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const libraries = useQuery(librariesQuery);
  const progress = useQuery(setupProgressQuery).data;
  const draft = useLibraryDraft();
  const [open, setOpen] = useState(false);
  const [missing, setMissing] = useState(false);
  const list = libraries.data ?? [];
  const advance = useMutation({
    mutationFn: async () =>
      queryClient.setQueryData(
        setupProgressQuery.queryKey,
        await updateSetupProgress({ step: 'libraries', state: 'done' }),
      ),
    onSuccess: () => navigate({ to: setupPath('household') }),
  });

  const add = () => {
    // A fresh draft starts with the wizard's default of «Vorschau beim Spulen».
    if (!draft.dirty && progress?.trickplayDefault) draft.setTrickplay(TRICKPLAY_ON);
    setOpen(true);
  };

  return (
    <SetupLayout
      step="libraries"
      intro={t('setup.steps.libraries.intro')}
      footer={{
        onNext: () => {
          if (list.length === 0) setMissing(true);
          else advance.mutate();
        },
        isPending: advance.isPending,
      }}
      error={advance.error ? problemMessage(t, advance.error) : undefined}
    >
      {libraries.isPending && <Skeleton shape="block" />}
      {libraries.isError && <Notice tone="danger">{problemMessage(t, libraries.error)}</Notice>}
      {list.map((library) => (
        <LibraryFacts key={library.id} library={library} />
      ))}
      {libraries.isSuccess && list.length === 0 && (
        <Notice tone={missing ? 'warning' : 'info'} live={missing}>
          {t('setup.libraries.none')}
        </Notice>
      )}
      <div className={styles.row}>
        <AriaButton variant={list.length === 0 ? 'primary' : 'secondary'} icon="plus" onPress={add}>
          {list.length === 0 ? t('setup.libraries.add') : t('setup.libraries.addMore')}
        </AriaButton>
      </div>
      <AddLibraryDialog
        draft={draft}
        isOpen={open}
        onOpenChange={setOpen}
        onCreated={() => setMissing(false)}
      />
    </SetupLayout>
  );
}
