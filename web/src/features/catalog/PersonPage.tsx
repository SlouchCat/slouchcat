import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ButtonLink,
  Notice,
  PosterCard,
  Row,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
} from '@/ui';
import { creditCaption } from './labels';
import { personQuery } from './queries';
import styles from './PersonPage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/people/$personId');

const BACK_KEY = 'person-back';

/** A person (B-3): name and the titles with a role that the active profile may see. */
export function PersonPage() {
  const { t } = useTranslation();
  const { personId } = route.useParams();
  const id = Number(personId);
  const valid = Number.isInteger(id) && id > 0;
  const query = useQuery({ ...personQuery(id), enabled: valid });
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  // No title while loading; "nicht gefunden" only once that is certain.
  useDocumentTitle(
    query.data?.name ?? (!valid || query.isError ? t('person.notFound.title') : undefined),
  );

  if (valid && query.isPending) {
    return (
      <main className={styles.page} aria-busy="true">
        <Skeleton shape="title" />
        <Skeleton shape="block" />
      </main>
    );
  }

  if (!valid || query.isError) {
    const notFound = !valid || isApiError(query.error, 404);
    return (
      <main className={styles.page}>
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
      </main>
    );
  }

  const person = query.data;
  if (!person) return null;
  const first = person.credits[0];
  return (
    <SpatialPage defaultFocusKey={first ? `person-credit-0` : BACK_KEY}>
      <main className={styles.page}>
        <div className={styles.head}>
          <h1 className={styles.name}>{person.name}</h1>
          <SpatialGroup>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
        </div>
        <div className={styles.rows}>
          <Row title={t('person.credits')} focusKey="person-credits">
            {person.credits.map((credit, index) => (
              <PosterCard
                key={`${credit.item.id}-${credit.role}`}
                focusKey={`person-credit-${index}`}
                href={paths.item(credit.item.id)}
                title={credit.item.title}
                caption={
                  creditCaption(t, credit) ??
                  (credit.item.year !== undefined ? String(credit.item.year) : undefined)
                }
                posterUrl={imageUrl(credit.item.images.poster)}
                posterSmallUrl={imageUrl(credit.item.images.posterSmall)}
                accent={credit.item.accent}
              />
            ))}
          </Row>
        </div>
      </main>
    </SpatialPage>
  );
}
