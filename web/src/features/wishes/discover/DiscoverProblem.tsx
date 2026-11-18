import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Button, ButtonLink, Notice, Skeleton } from '@/ui';
import { wishCountsQuery } from '../queries';
import { wishProblemMessage } from '../state';

/**
 * The notice's title that also names the tab: only a page that is not there renames it; on any
 * other problem the tab keeps the page's own title.
 */
function GoneTitle({ title }: { title: string }) {
  useDocumentTitle(title);
  return null;
}

/**
 * What a page of the area shows when its list could not be read (concept 25, 5.4, M-31):
 * 404 (no right, or the pages are off) is «Entdecken gibt es für dieses Profil nicht» with the way
 * to «Meine Wünsche» for a profile that has wishes, and «not found» for any other; every other
 * problem (Seerr out of reach with nothing kept, too many calls) in the viewer's words with a
 * retry. The server says which (rule 12).
 */
export function DiscoverProblem({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const gone = isApiError(error, 404);
  const counts = useQuery({
    ...wishCountsQuery,
    enabled: gone && session.wishes?.enabled === true,
  });
  const hasWishes = counts.data
    ? counts.data.open + counts.data.available + counts.data.closed > 0
    : false;
  if (!gone) {
    return (
      <Notice
        tone="danger"
        title={wishProblemMessage(t, error)}
        actions={
          <Button icon="refresh" onPress={onRetry}>
            {t('common.retry')}
          </Button>
        }
      />
    );
  }
  if (counts.isPending && counts.fetchStatus !== 'idle') return <Skeleton shape="block" />;
  if (hasWishes) {
    return (
      <>
        <GoneTitle title={t('wishes.discover.gone.title')} />
        <Notice
          tone="info"
          title={t('wishes.discover.gone.title')}
          actions={
            <ButtonLink href={paths.wishes} variant="primary" icon="heart">
              {t('wishes.discover.gone.action')}
            </ButtonLink>
          }
        >
          {t('wishes.discover.gone.body')}
        </Notice>
      </>
    );
  }
  return (
    <>
      <GoneTitle title={t('wishes.discover.notFound.title')} />
      <Notice
        tone="info"
        title={t('wishes.discover.notFound.title')}
        actions={
          <ButtonLink href={paths.home} variant="primary">
            {t('item.notFound.action')}
          </ButtonLink>
        }
      >
        {t('wishes.discover.notFound.body')}
      </Notice>
    </>
  );
}
