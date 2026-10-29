import type { MetadataProvider, MetadataProviderInfo, RootContent } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useLayoutEffect, useRef } from 'react';
import { Checkbox } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import { Chip } from './Chip';
import { Icon } from './Icon';
import fieldStyles from './Field.module.css';
import styles from './ProviderList.module.css';

/** The order in which the providers are offered when nothing is ticked yet. */
const PROVIDER_ORDER: readonly MetadataProvider[] = ['tmdb', 'tvdb', 'anilist', 'local'];

/** « › » between the names of a list: "TVDB › TMDB › AniList". */
export function providerChain(t: TFunction, providers: readonly MetadataProvider[]): string {
  return providers.map((id) => t(`library.provider.${id}`)).join(t('library.providers.chain'));
}

/**
 * The providers a folder of this content may carry, in the order of the list (the ticked ones
 * first, in their order, then the rest as the server names them). Without `infos` only the
 * ticked ones are known.
 */
export function allowedProviders(
  content: RootContent,
  value: readonly MetadataProvider[],
  infos: readonly MetadataProviderInfo[] | undefined,
): MetadataProvider[] {
  const allowed = infos
    ? infos.filter((info) => info.contents.includes(content)).map((info) => info.id)
    : [...value];
  const ordered = value.filter((id) => allowed.includes(id));
  for (const id of PROVIDER_ORDER) {
    if (allowed.includes(id) && !ordered.includes(id)) ordered.push(id);
  }
  return ordered;
}

/** Whether a provider may stand first in the list of a folder of this content. */
export function canIdentify(
  content: RootContent,
  id: MetadataProvider,
  infos: readonly MetadataProviderInfo[] | undefined,
): boolean {
  const info = infos?.find((entry) => entry.id === id);
  // Without the server's word the forms send the default lists, whose first entry identifies.
  if (!info) return true;
  return info.contents.includes(content) && info.roles.includes('identify');
}

/**
 * What is wrong with the list before the server sees it: nothing ticked, or a provider in front
 * that only fills in (the server answers 422 providers_not_allowed to the same). The client
 * checks no rule (CLAUDE.md rule 12); it only does not offer the impossible.
 */
export function providerListError(
  t: TFunction,
  content: RootContent,
  value: readonly MetadataProvider[],
  infos: readonly MetadataProviderInfo[] | undefined,
): string | null {
  const [first] = value;
  if (!first) return t('library.providers.required');
  if (canIdentify(content, first, infos)) return null;
  const identifiers = allowedProviders(content, [], infos).filter((id) =>
    canIdentify(content, id, infos),
  );
  return t('library.providers.cannotIdentify', {
    provider: t(`library.provider.${first}`),
    identifiers: identifiers.map((id) => t(`library.provider.${id}`)).join(t('format.or')),
  });
}

interface ProviderListProps {
  /** The content of the folder: which providers are offered and which may identify. */
  content: RootContent;
  /** The ticked providers, in their order; the first one identifies. */
  value: readonly MetadataProvider[];
  onChange: (value: MetadataProvider[]) => void;
  /** What `listMetadataProviders` said; without it the list is shown fixed (the setup wizard). */
  infos?: readonly MetadataProviderInfo[];
  /** Name of the group, for the checkboxes (`roots[0].providers`). */
  name?: string;
  label?: string;
  /** A message of the server about this list (422 providers_not_allowed). */
  error?: string;
}

/**
 * «Metadatenquellen» of a folder (ADR 0015, concept 7): one row per provider the content allows,
 * with a checkbox, the name, what the entry does («erkennt Titel» for the first ticked one,
 * «füllt auf» for the others), a hint («kein Schlüssel hinterlegt», «liefert kein Deutsch», «nur
 * Auffüller») and two arrow buttons («TMDB nach oben» / «TMDB nach unten», icon only). The order
 * is changed with those buttons or with the arrow keys on a focused one, like the rows of the
 * start page: rule 8 asks for the keyboard, not for drag and drop. The focus stays on the pressed
 * button after the rows swapped.
 *
 * A content with one provider (audiobooks, podcasts) shows it as text, nothing to tick. Without
 * `infos` the list is text as well: the setup wizard has no session to ask the server with.
 */
export function ProviderList({
  content,
  value,
  onChange,
  infos,
  name,
  label,
  error,
}: ProviderListProps) {
  const { t } = useTranslation();
  const providers = allowedProviders(content, value, infos);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const pending = useRef<{ id: MetadataProvider; delta: -1 | 1 } | null>(null);
  const order = providers.join();
  useLayoutEffect(() => {
    const move = pending.current;
    if (!move) return;
    pending.current = null;
    const pressed = buttons.current.get(`${move.id}:${move.delta}`);
    const other = buttons.current.get(`${move.id}:${-move.delta}`);
    (pressed && !pressed.disabled ? pressed : other)?.focus();
  }, [order]);

  const title = label ?? t('library.providers.title');
  const first = value[0];
  const own = providerListError(t, content, value, infos);
  const message = error ?? own;

  if (!infos || providers.length <= 1) {
    const fixed = providers[0];
    return (
      <div className={fieldStyles.field}>
        <span className={fieldStyles.label}>{title}</span>
        <p className={styles.fixed}>
          <strong>
            {value.length > 0 ? providerChain(t, value) : fixed && t(`library.provider.${fixed}`)}
          </strong>
          <span className={fieldStyles.description}>
            {infos ? t('library.providers.fixedHint') : t('library.providers.defaultHint')}
          </span>
        </p>
      </div>
    );
  }

  // Only the ticked rows have an order (the list the server gets); the unticked ones follow in
  // the order the server names them, so their buttons are disabled.
  const tickedOrder = providers.filter((entry) => value.includes(entry));
  const move = (id: MetadataProvider, delta: -1 | 1) => {
    const index = tickedOrder.indexOf(id);
    const target = index + delta;
    if (index < 0 || target < 0 || target >= tickedOrder.length) return;
    const next = [...tickedOrder];
    [next[index], next[target]] = [next[target]!, next[index]!];
    pending.current = { id, delta };
    onChange(next);
  };
  const arrows = (id: MetadataProvider) => (event: { key: string; preventDefault: () => void }) => {
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
    event.preventDefault(); // the page must not scroll while the row moves
    move(id, event.key === 'ArrowUp' ? -1 : 1);
  };
  const buttonRef = (id: MetadataProvider, delta: -1 | 1) => (node: HTMLButtonElement | null) => {
    if (node) buttons.current.set(`${id}:${delta}`, node);
    else buttons.current.delete(`${id}:${delta}`);
  };

  return (
    <fieldset className={`${fieldStyles.field} ${styles.fieldset}`} name={name}>
      <legend className={fieldStyles.label}>{title}</legend>
      <ul className={styles.list}>
        {providers.map((id) => {
          const info = infos.find((entry) => entry.id === id);
          const ticked = value.includes(id);
          const position = tickedOrder.indexOf(id);
          const identifies = canIdentify(content, id, infos);
          const providerName = t(`library.provider.${id}`);
          const hints = [
            info && !info.configured && t('library.providers.noKey'),
            info && !info.languages.includes('de') && t('library.providers.noGerman'),
            !identifies && t('library.providers.fillOnly'),
          ].filter(Boolean);
          return (
            <li key={id} className={`${styles.row} ${ticked ? '' : styles.off}`}>
              <Checkbox
                className={styles.option}
                isSelected={ticked}
                onChange={(selected) => {
                  // Ticking keeps the shown order; unticking drops the entry from the list.
                  onChange(
                    providers.filter((entry) => (entry === id ? selected : value.includes(entry))),
                  );
                }}
                aria-label={providerName}
              >
                <span className={styles.box} aria-hidden="true">
                  <Icon name="check" size="sm" />
                </span>
              </Checkbox>
              <span className={styles.text}>
                <span className={styles.name}>
                  {providerName}
                  {ticked && id === first && identifies && (
                    <Chip tone="success">{t('library.providers.identifies')}</Chip>
                  )}
                  {ticked && id === first && !identifies && (
                    <Chip tone="danger">{t('library.providers.cannotIdentifyChip')}</Chip>
                  )}
                  {ticked && id !== first && (
                    <Chip tone="info">{t('library.providers.fills')}</Chip>
                  )}
                </span>
                {hints.length > 0 && (
                  <span className={styles.hint}>{hints.join(t('format.separator'))}</span>
                )}
              </span>
              <span className={styles.sort}>
                <AriaButton
                  ref={buttonRef(id, -1)}
                  variant="ghost"
                  icon="chevronUp"
                  iconOnly
                  aria-label={t('library.providers.moveUpLabel', { provider: providerName })}
                  isDisabled={position <= 0}
                  onPress={() => move(id, -1)}
                  onKeyDown={arrows(id)}
                />
                <AriaButton
                  ref={buttonRef(id, 1)}
                  variant="ghost"
                  icon="chevronDown"
                  iconOnly
                  aria-label={t('library.providers.moveDownLabel', { provider: providerName })}
                  isDisabled={position < 0 || position === tickedOrder.length - 1}
                  onPress={() => move(id, 1)}
                  onKeyDown={arrows(id)}
                />
              </span>
            </li>
          );
        })}
      </ul>
      <span className={fieldStyles.description}>{t('library.providers.hint')}</span>
      {message && (
        <span role="alert" className={fieldStyles.error}>
          {message}
        </span>
      )}
    </fieldset>
  );
}
