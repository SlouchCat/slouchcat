import type {
  LibraryKind,
  MetadataProvider,
  MetadataProviderInfo,
  RootContent,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useEffect, useRef, type ReactNode, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import { Chip } from './Chip';
import { ProviderList, providerChain, providerListError } from './ProviderList';
import { RadioGroup } from './RadioGroup';
import { TextField } from './TextField';
import styles from './FolderList.module.css';

/** One folder of the list, as the forms hold it before it is sent. */
export interface FolderEntry {
  /** Stable key of the card (not its position: cards go in and out). */
  key: string;
  path: string;
  content: RootContent;
  providers: MetadataProvider[];
}

/** The contents a library of this kind may hold in its folders (concept 07, F-3). */
export function contentsOf(kind: LibraryKind): RootContent[] {
  if (kind === 'audiobooks') return ['audiobook'];
  if (kind === 'podcasts') return ['podcast'];
  return ['movie', 'series'];
}

/** The list a new folder of this content starts with: the order the server used before. */
export function defaultProviders(content: RootContent): MetadataProvider[] {
  if (content === 'series') return ['tmdb', 'tvdb'];
  if (content === 'audiobook' || content === 'podcast') return ['local'];
  return ['tmdb'];
}

let sequence = 0;

/** Focus target of a removed card: the add button (no card key starts with a space). */
const ADD_BUTTON = ' add';

/**
 * A fresh card. In a video library the second card offers the other content, so «Dokus» with a
 * film and a series folder is two clicks (concept 7).
 */
export function newFolderEntry(kind: LibraryKind, existing: readonly FolderEntry[]): FolderEntry {
  const contents = contentsOf(kind);
  const used = new Set(existing.map((entry) => entry.content));
  const content = contents.find((candidate) => !used.has(candidate)) ?? contents[0] ?? 'movie';
  sequence += 1;
  return { key: `folder-${sequence}`, path: '', content, providers: defaultProviders(content) };
}

/** Whether one path lies in the other (or is the same), on a path boundary. */
function overlaps(a: string, b: string): boolean {
  const x = a.replace(/\/+$/, '') || '/';
  const y = b.replace(/\/+$/, '') || '/';
  if (x === y) return true;
  const [short, long] = x.length < y.length ? [x, y] : [y, x];
  return short === '/' || long.startsWith(`${short}/`);
}

/**
 * What keeps the list from being sent: two cards whose paths overlap, or a provider list that
 * cannot identify (the server would answer 422 to both). A blank path is the field's own
 * business (it is required), so an untouched card does not block the button. The message of the
 * first problem; null when the list is fine.
 */
export function folderListError(
  t: TFunction,
  entries: readonly FolderEntry[],
  infos: readonly MetadataProviderInfo[] | undefined,
): string | null {
  for (const [index, entry] of entries.entries()) {
    const path = entry.path.trim();
    const earlier =
      path && entries.slice(0, index).find((other) => overlaps(other.path.trim(), path));
    if (earlier) return t('library.roots.overlap', { number: index + 1 });
    const providers = providerListError(t, entry.content, entry.providers, infos);
    if (providers) return t('library.roots.providers', { number: index + 1, message: providers });
  }
  return null;
}

interface FolderListProps {
  kind: LibraryKind;
  entries: readonly FolderEntry[];
  onChange: (entries: FolderEntry[]) => void;
  /** What `listMetadataProviders` said; absent in the setup wizard (fixed lists then). */
  infos?: readonly MetadataProviderInfo[];
  /** How many cards the list keeps at least: one for video and audiobooks, none for podcasts. */
  min: number;
  /** Prefix of the input names: `roots` on the admin page, `library.roots` in the wizard. */
  namePrefix: string;
  /** Rendered right of the path field of a card: the browse button. */
  pathButton?: (entry: FolderEntry, inputRef: RefObject<HTMLInputElement | null>) => ReactNode;
  /** Rendered below the path field of a card: the wizard's check. */
  pathAddon?: (entry: FolderEntry, inputRef: RefObject<HTMLInputElement | null>) => ReactNode;
  /** The path field of a card lost the focus (the wizard checks the folder then). */
  onPathBlur?: (entry: FolderEntry) => void;
  /** The server's field messages by input name (`roots[1].providers`), shown at the field. */
  fieldErrors?: Readonly<Record<string, string>>;
}

/**
 * «Ordnerliste» (ADR 0015, F-13): the folders of a library as numbered cards, each with path,
 * «Inhalt» (films or series in a video library; fixed otherwise) and «Metadatenquellen», and
 * «+ Weiteren Ordner hinzufügen» below. One submit of the surrounding form creates them all, the
 * server all of them or none. Removing a card only edits the form. The new card takes the focus
 * in its path field; removing one hands it to the add button, so the keyboard never loses its
 * place (rule 8).
 */
export function FolderList({
  kind,
  entries,
  onChange,
  infos,
  min,
  namePrefix,
  pathButton,
  pathAddon,
  onPathBlur,
  fieldErrors,
}: FolderListProps) {
  const { t } = useTranslation();
  const contents = contentsOf(kind);
  const inputs = useRef(new Map<string, HTMLInputElement>());
  const addButton = useRef<HTMLButtonElement>(null);
  // The card key whose path field takes the focus after the next render, or the add button.
  const focusNext = useRef<string | null>(null);
  useEffect(() => {
    const target = focusNext.current;
    if (!target) return;
    focusNext.current = null;
    if (target === ADD_BUTTON) addButton.current?.focus();
    else inputs.current.get(target)?.focus();
  }, [entries]);

  const update = (key: string, patch: Partial<FolderEntry>) =>
    onChange(entries.map((entry) => (entry.key === key ? { ...entry, ...patch } : entry)));
  const add = () => {
    const entry = newFolderEntry(kind, entries);
    focusNext.current = entry.key;
    onChange([...entries, entry]);
  };
  const remove = (key: string) => {
    focusNext.current = ADD_BUTTON;
    onChange(entries.filter((entry) => entry.key !== key));
  };

  return (
    <div className={styles.list}>
      {entries.length === 0 && (
        <p className={styles.empty}>
          {kind === 'podcasts' ? t('library.roots.emptyPodcasts') : t('library.roots.empty')}
        </p>
      )}
      <ol className={styles.cards}>
        {entries.map((entry, index) => {
          const number = index + 1;
          const name = `${namePrefix}[${index}]`;
          const inputRef: RefObject<HTMLInputElement | null> = {
            get current() {
              return inputs.current.get(entry.key) ?? null;
            },
            set current(node) {
              if (node) inputs.current.set(entry.key, node);
              else inputs.current.delete(entry.key);
            },
          };
          return (
            <li
              key={entry.key}
              className={styles.card}
              aria-label={t('library.roots.number', { number })}
            >
              <div className={styles.head}>
                <span className={styles.number} aria-hidden="true">
                  {number}
                </span>
                <span className={styles.summary}>
                  <span className={styles.cardTitle}>{t('library.roots.number', { number })}</span>
                  <Chip tone="info">{t(`library.content.${entry.content}`)}</Chip>
                  <span className={styles.chain}>{providerChain(t, entry.providers)}</span>
                </span>
                {entries.length > min && (
                  <AriaButton
                    variant="ghost"
                    icon="trash"
                    aria-label={t('library.roots.removeLabel', { number })}
                    onPress={() => remove(entry.key)}
                  >
                    {t('library.roots.remove')}
                  </AriaButton>
                )}
              </div>
              <div className={styles.fields}>
                <div className={styles.pathRow}>
                  <TextField
                    name={`${name}.path`}
                    label={t('setup.library.path')}
                    placeholder={t('setup.library.pathPlaceholder')}
                    autoComplete="off"
                    spellCheck="false"
                    mono
                    isRequired
                    inputRef={inputRef}
                    value={entry.path}
                    onChange={(path) => update(entry.key, { path })}
                    onBlur={onPathBlur ? () => onPathBlur(entry) : undefined}
                    validate={(value) => {
                      if (!value.trim()) return t('errors.field.required');
                      return value.trim().startsWith('/') ? null : t('errors.field.not_absolute');
                    }}
                  />
                  {pathButton && (
                    <div className={styles.pathButton}>{pathButton(entry, inputRef)}</div>
                  )}
                </div>
                {pathAddon?.(entry, inputRef)}
                {contents.length > 1 && (
                  <RadioGroup
                    name={`${name}.content`}
                    label={t('library.content.title')}
                    description={t('library.content.hint')}
                    value={entry.content}
                    onChange={(value) => {
                      const content = contents.find((candidate) => candidate === value);
                      if (content && content !== entry.content) {
                        update(entry.key, { content, providers: defaultProviders(content) });
                      }
                    }}
                    options={contents.map((value) => ({
                      value,
                      label: t(`library.content.${value}`),
                    }))}
                  />
                )}
                <ProviderList
                  name={`${name}.providers`}
                  content={entry.content}
                  value={entry.providers}
                  onChange={(providers) => update(entry.key, { providers })}
                  infos={infos}
                  error={fieldErrors?.[`${name}.providers`] ?? fieldErrors?.[`${name}.content`]}
                />
              </div>
            </li>
          );
        })}
      </ol>
      <div className={styles.add}>
        <AriaButton ref={addButton} icon="plus" onPress={add}>
          {entries.length === 0 ? t('library.roots.addFirst') : t('library.roots.addMore')}
        </AriaButton>
      </div>
    </div>
  );
}
