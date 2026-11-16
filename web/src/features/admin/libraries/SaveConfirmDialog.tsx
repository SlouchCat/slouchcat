import type { Library, LibraryRoot } from '@slouchcat/api-client';
import type { FormEvent, ReactNode } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, TextField } from '@/ui/aria';
import type { Confirmation } from './libraryForm';
import styles from './Libraries.module.css';

/**
 * What removing a folder really does. It is longer than a confirmation usually is on purpose:
 * "delete" is both too frightening (the titles are soft-deleted and return with their user
 * data once they come back with the same provider id, ADR 0023) and too harmless (the offline
 * packages on the devices are gone, a running playback breaks off, and the path map of Radarr
 * and Sonarr points nowhere afterwards). `last`: the library keeps no folder after the save.
 */
export function RemovalConsequences({
  library,
  root,
  last,
}: {
  library: Library;
  root: LibraryRoot;
  last: boolean;
}) {
  const { t } = useTranslation();
  return (
    <>
      <p>
        <Trans
          i18nKey="admin.libraries.folders.remove.lead"
          values={{ path: root.path, name: library.name, count: root.itemCount }}
          components={{ path: <code /> }}
        />
      </p>
      <ul>
        <li>{t('admin.libraries.folders.remove.soft')}</li>
        <li>{t('admin.libraries.folders.remove.downloads')}</li>
        <li>{t('admin.libraries.folders.remove.playback')}</li>
        <li>{t('admin.libraries.folders.remove.webhooks')}</li>
        <li>{t('admin.libraries.folders.remove.relink')}</li>
        <li>{t('admin.libraries.folders.remove.oneWay')}</li>
        {last && <li>{t('admin.libraries.folders.remove.last', { name: library.name })}</li>}
      </ul>
    </>
  );
}

interface SaveConfirmDialogProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  library: Library;
  confirmations: readonly Confirmation[];
  /** The library keeps no folder after the save: the last removal says so. */
  last: boolean;
  /** What was typed per card key; it stays while the dialog is open and the save is refused. */
  typed: Readonly<Record<string, string>>;
  onTypedChange: (key: string, value: string) => void;
  /** The server's messages of the typed fields (`confirm.<key>`). */
  fieldErrors?: Readonly<Record<string, string>>;
  /** A refusal of the whole save that the dialog shows (409 `scan_running` with its action). */
  problem?: ReactNode;
  isPending: boolean;
  onConfirm: () => void;
}

/**
 * «Änderungen bestätigen» (concept 24 4.3): before the save, every heavy change of the form, one
 * section per folder. Removing a folder and reading it as another content name their
 * consequences (the text of RemovalConsequences) and have a field each in which the folder's path
 * is typed out, character for character (idea 02, E12); a new path names the old and the new one
 * and needs no typed path. «Speichern» stays disabled until every field repeats its path; the
 * server compares them again against its own rows.
 */
export function SaveConfirmDialog({
  isOpen,
  onOpenChange,
  library,
  confirmations,
  last,
  typed,
  onTypedChange,
  fieldErrors,
  problem,
  isPending,
  onConfirm,
}: SaveConfirmDialogProps) {
  const { t } = useTranslation();
  const needsPath = confirmations.filter((entry) => entry.kind !== 'path');
  const matches = needsPath.every((entry) => (typed[entry.key] ?? '').trim() === entry.root.path);
  const lastRemoval = [...confirmations].reverse().find((entry) => entry.kind === 'remove');
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (matches && !isPending) onConfirm();
  };

  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      title={t('admin.libraries.form.confirm.title')}
      description={t('admin.libraries.form.confirm.intro')}
      size="wide"
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={fieldErrors}>
        {problem}
        {confirmations.map((entry) => (
          <section key={entry.key} className={styles.confirmFolder} aria-label={entry.root.path}>
            <h3 className={styles.rootsTitle}>
              {t(`admin.libraries.form.confirm.${entry.kind}`, { path: entry.root.path })}
            </h3>
            {entry.kind === 'path' ? (
              <p>
                {t('admin.libraries.folders.move.body', { old: entry.root.path, new: entry.path })}
              </p>
            ) : (
              <>
                {entry.kind === 'content' && <p>{t('admin.libraries.folders.change.content')}</p>}
                <RemovalConsequences
                  library={library}
                  root={entry.root}
                  last={last && entry === lastRemoval}
                />
                <TextField
                  name={`confirm.${entry.key}`}
                  label={t('admin.libraries.folders.remove.inputLabel')}
                  value={typed[entry.key] ?? ''}
                  onChange={(value) => onTypedChange(entry.key, value)}
                  isReadOnly={isPending}
                  mono
                  autoComplete="off"
                  spellCheck="false"
                  description={
                    (typed[entry.key] ?? '').trim() === entry.root.path
                      ? undefined
                      : t('admin.libraries.folders.remove.mismatch', { path: entry.root.path })
                  }
                />
              </>
            )}
          </section>
        ))}
        {!matches && <Notice live={false}>{t('admin.libraries.form.confirm.typeAll')}</Notice>}
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={() => onOpenChange(false)}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant={needsPath.length > 0 ? 'danger' : 'primary'}
            isDisabled={!matches}
            isPending={isPending}
          >
            {t('admin.libraries.form.confirm.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
