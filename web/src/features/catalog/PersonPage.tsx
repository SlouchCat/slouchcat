import type { ItemRef, Person } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useId, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { formatLongDay, yearsBetween } from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Backdrop,
  Button,
  ButtonLink,
  FactList,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
} from '@/ui';
import { cardKey, ItemGrid } from '@/features/discover/ItemGrid';
import { useCoverPens } from './EditMetadata';
import { creditCaption } from './labels';
import { personQuery } from './queries';
import styles from './PersonPage.module.css';
import { intlLocale } from '@/i18n';

const route = getRouteApi('/_authed/_active/_chrome/people/$personId');

const BACK_KEY = 'person-back';
const GRID_KEY = 'person-credit';

/** The TMDB departments (`Person.knownForDepartment`) the locale names; any other shows as it comes. */
const DEPARTMENTS = {
  Acting: 'person.department.acting',
  Directing: 'person.department.directing',
  Writing: 'person.department.writing',
  Production: 'person.department.production',
  Sound: 'person.department.sound',
  Camera: 'person.department.camera',
  Editing: 'person.department.editing',
  Art: 'person.department.art',
  Crew: 'person.department.crew',
  'Costume & Make-Up': 'person.department.costume',
  'Visual Effects': 'person.department.visualEffects',
  Lighting: 'person.department.lighting',
  Creator: 'person.department.creator',
} as const;

/** A title of the person with every role in it, in the order of the credits. */
interface Title {
  item: ItemRef;
  roles: string[];
}

/** The credits as titles: a person who acted in and directed a film has one card for it. */
function titlesOf(t: TFunction, credits: Person['credits']): Title[] {
  const byId = new Map<string, Title>();
  for (const credit of credits) {
    const title = byId.get(credit.item.id) ?? { item: credit.item, roles: [] };
    const role = creditCaption(t, credit);
    if (role !== undefined && !title.roles.includes(role)) title.roles.push(role);
    byId.set(credit.item.id, title);
  }
  return [...byId.values()];
}

/** "Film · 2004 · als Moe, Regie": the caption of a title on the person page (concept 21 P-8). */
function titleCaption(t: TFunction, title: Title): string {
  return [
    t(`kind.${title.item.kind}`),
    title.item.year !== undefined ? String(title.item.year) : undefined,
    title.roles.length > 0 ? title.roles.join(', ') : undefined,
  ]
    .filter((part): part is string => part !== undefined)
    .join(' · ');
}

/** The facts beside the name, in the order of the concept; a fact TMDB did not give is left out. */
function personFacts(
  t: TFunction,
  language: string,
  person: Person,
  shown: number,
): (readonly [string, string])[] {
  const today = new Date().toISOString().slice(0, 10);
  const facts: (readonly [string, string])[] = [];
  const department = person.knownForDepartment;
  if (department) {
    const key = Object.hasOwn(DEPARTMENTS, department)
      ? DEPARTMENTS[department as keyof typeof DEPARTMENTS]
      : undefined;
    facts.push([t('person.facts.knownFor'), key ? t(key) : department]);
  }
  if (person.birthday) {
    const date = formatLongDay(person.birthday, language);
    facts.push([
      t('person.facts.born'),
      person.deathday
        ? date
        : t('person.facts.bornAge', { date, age: yearsBetween(person.birthday, today) }),
    ]);
  }
  if (person.placeOfBirth) facts.push([t('person.facts.birthplace'), person.placeOfBirth]);
  if (person.deathday) {
    const date = formatLongDay(person.deathday, language);
    facts.push([
      t('person.facts.died'),
      person.birthday
        ? t('person.facts.diedAge', { date, age: yearsBetween(person.birthday, person.deathday) })
        : date,
    ]);
  }
  facts.push([
    t('person.facts.library'),
    person.totalCredits !== undefined
      ? t('person.facts.libraryOf', { shown, count: Math.max(person.totalCredits, shown) })
      : t('person.facts.libraryOnly', { count: shown }),
  ]);
  return facts;
}

/**
 * The biography in three lines with «Mehr lesen» when it is longer, and the hint when it is the
 * English one in place of a missing one in the main language (decided by the server, O-13).
 */
function Biography({ person }: { person: Person }) {
  const { t } = useTranslation();
  const id = useId();
  const text = useRef<HTMLParagraphElement>(null);
  const [open, setOpen] = useState(false);
  const [clamped, setClamped] = useState(false);
  // Whether three lines cut the text; measured while closed, again when the width changes.
  useLayoutEffect(() => {
    const node = text.current;
    if (!node || open) return;
    const update = () => setClamped(node.scrollHeight > node.clientHeight + 1);
    update();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(node);
    return () => observer.disconnect();
  }, [open, person.biography]);
  return (
    <div className={styles.biography}>
      <p
        ref={text}
        id={id}
        lang={person.biographyLang}
        className={open ? styles.biographyText : `${styles.biographyText} ${styles.clamped}`}
      >
        {person.biography}
      </p>
      {(clamped || open || person.biographyFallback) && (
        <div className={styles.biographyBar}>
          {(clamped || open) && (
            <SpatialGroup>
              <SpatialButton
                focusKey="person-biography"
                variant="ghost"
                aria-expanded={open}
                aria-controls={id}
                onPress={() => setOpen((value) => !value)}
              >
                {open ? t('person.biography.less') : t('person.biography.more')}
              </SpatialButton>
            </SpatialGroup>
          )}
          {person.biographyFallback && (
            <p className={styles.hint}>{t('person.biography.fallback')}</p>
          )}
        </div>
      )}
    </div>
  );
}

/** The 2:3 portrait, or the initial on a neutral ground without one or when it fails (as CastCard). */
function Portrait({ name, url }: { name: string; url?: string }) {
  // The URL that failed, so a new URL (a refreshed hash) gets its own try.
  const [failed, setFailed] = useState<string>();
  const shown = url !== undefined && url !== failed;
  return (
    <span className={styles.portrait} aria-hidden="true">
      {shown ? (
        <img
          className={styles.portraitImage}
          src={url}
          alt=""
          width={300}
          height={450}
          onError={() => setFailed(url)}
        />
      ) : (
        Array.from(name)[0]?.toUpperCase()
      )}
    </span>
  );
}

/**
 * A person (B-3, concept 21 P-8): a band of the blurred portrait behind the head, the 2:3 portrait
 * beside the name, the facts and the biography from TMDB; under it «Zurück» and the titles with a
 * role that the active profile may see, over the full width.
 */
export function PersonPage() {
  const { t, i18n } = useTranslation();
  const pens = useCoverPens();
  const { personId } = route.useParams();
  const id = Number(personId);
  const valid = Number.isInteger(id) && id > 0;
  const query = useQuery({ ...personQuery(id), enabled: valid });
  const goBack = useGoBack(paths.home);
  const headingId = useId();
  useBackKeys(goBack);
  // No title while loading; "nicht gefunden" only once that is certain.
  useDocumentTitle(
    query.data?.name ?? (!valid || query.isError ? t('person.notFound.title') : undefined),
  );

  if (valid && query.isPending) {
    return (
      <main className={styles.page} aria-busy="true">
        <div className={styles.block}>
          <Skeleton shape="title" />
          <Skeleton shape="block" />
        </div>
      </main>
    );
  }

  if (!valid || query.isError) {
    const notFound = !valid || isApiError(query.error, 404);
    return (
      <main className={styles.page}>
        <div className={styles.block}>
          <Notice
            tone={notFound ? 'info' : 'danger'}
            title={notFound ? t('person.notFound.title') : problemMessage(t, query.error)}
            actions={
              notFound ? (
                <ButtonLink href={paths.home} variant="primary">
                  {t('item.notFound.action')}
                </ButtonLink>
              ) : (
                <Button icon="refresh" onPress={() => void query.refetch()}>
                  {t('common.retry')}
                </Button>
              )
            }
          >
            {notFound && t('person.notFound.body')}
          </Notice>
        </div>
      </main>
    );
  }

  const person = query.data;
  if (!person) return null;
  const titles = titlesOf(t, person.credits);
  const captions = new Map(titles.map((title) => [title.item.id, titleCaption(t, title)]));
  const portrait = imageUrl(person.image);
  const first = titles[0];
  return (
    <SpatialPage defaultFocusKey={first ? cardKey(GRID_KEY, first.item) : BACK_KEY}>
      <main className={styles.page}>
        {portrait && <Backdrop posterUrl={portrait} />}
        <div className={styles.head}>
          <Portrait name={person.name} url={portrait} />
          <div className={styles.info}>
            <h1 className={styles.name}>{person.name}</h1>
            <FactList facts={personFacts(t, intlLocale(i18n.language), person, titles.length)} />
            {person.biography && <Biography person={person} />}
          </div>
        </div>
        <SpatialGroup className={styles.block}>
          <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
            {t('common.back')}
          </SpatialButton>
        </SpatialGroup>
        {titles.length > 0 && (
          <section className={styles.credits} aria-labelledby={headingId}>
            <h2 id={headingId} className={styles.creditsTitle}>
              {t('person.credits')}{' '}
              <span className={styles.muted}>
                {t('person.creditsCount', { count: titles.length })}
              </span>
            </h2>
            <ItemGrid
              items={titles.map((title) => title.item)}
              focusKey={GRID_KEY}
              label={t('person.credits')}
              tool={pens.tool}
              describe={(item) => ({ caption: captions.get(item.id) })}
            />
          </section>
        )}
        {pens.editor}
      </main>
    </SpatialPage>
  );
}
