import type { ItemDetail, MediaFileInfo } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { apiUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { formatBytes, formatClock, resolutionClass } from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Backdrop,
  Button,
  ButtonLink,
  Chip,
  Notice,
  PosterFallback,
  ProgressBar,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  itemAccentStyle,
} from '@/ui';
import { ItemMeta } from './ItemMeta';
import { itemQuery } from './queries';
import styles from './ItemPage.module.css';

const route = getRouteApi('/_authed/_chrome/items/$itemId');

const PLAY_KEY = 'detail-play';
// Rendered poster width (ItemPage.module.css, wide layout only).
const POSTER_SIZES = 'clamp(13rem, 20vw, 18rem)';

const imageUrl = (image: { url: string } | undefined) => (image ? apiUrl(image.url) : undefined);
const BACK_KEY = 'detail-back';

function FileFacts({ file, t, language }: { file: MediaFileInfo; t: TFunction; language: string }) {
  const facts: [string, string][] = [[t('item.file.container'), file.container.toUpperCase()]];
  if (file.width && file.height) {
    facts.push([
      t('item.file.resolution'),
      `${t('format.resolution', { width: file.width, height: file.height })} · ${resolutionClass(t, file.width, file.height)}`,
    ]);
  }
  if (file.videoCodec) facts.push([t('item.file.video'), file.videoCodec.toUpperCase()]);
  if (file.audioCodec) facts.push([t('item.file.audio'), file.audioCodec.toUpperCase()]);
  facts.push([t('item.file.size'), formatBytes(file.sizeBytes, language)]);
  if (file.versionLabel) facts.push([t('item.file.version'), file.versionLabel]);

  return (
    <div className={styles.file}>
      {!file.available && (
        <Chip tone="warning" icon="warning">
          {t('item.file.unavailable')}
        </Chip>
      )}
      <dl className={styles.facts}>
        {facts.map(([term, value]) => (
          <div key={term} className={styles.fact}>
            <dt>{term}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function Actions({ item, onBack }: { item: ItemDetail; onBack: () => void }) {
  const { t } = useTranslation();
  const playable = item.mediaFiles.some((file) => file.available);
  const resume =
    item.progress && !item.progress.completed && item.progress.positionMs > 0
      ? item.progress
      : undefined;
  const playLabel = resume
    ? t('item.resume', { time: formatClock(resume.positionMs) })
    : t('item.play');

  return (
    <SpatialGroup className={styles.actions}>
      {playable ? (
        <SpatialButtonLink
          focusKey={PLAY_KEY}
          href={paths.play(item.id)}
          variant="primary"
          size="lg"
          icon="play"
        >
          {playLabel}
        </SpatialButtonLink>
      ) : (
        <Button variant="primary" size="lg" icon="play" isDisabled>
          {playLabel}
        </Button>
      )}
      <SpatialButton focusKey={BACK_KEY} variant="ghost" size="lg" icon="back" onPress={onBack}>
        {t('common.back')}
      </SpatialButton>
    </SpatialGroup>
  );
}

/** Detail page of a title: artwork, facts, overview, file information, play or resume. */
export function ItemPage() {
  const { t, i18n } = useTranslation();
  const { itemId } = route.useParams();
  const query = useQuery(itemQuery(itemId));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  useDocumentTitle(query.data?.title);

  if (query.isPending) {
    return (
      <main className={styles.detail} aria-busy="true">
        <div className={styles.layout}>
          <div className={styles.poster}>
            <Skeleton shape="poster" />
          </div>
          <div className={styles.info}>
            <Skeleton shape="title" />
            <Skeleton shape="line" />
            <Skeleton shape="block" />
          </div>
        </div>
      </main>
    );
  }

  if (query.isError) {
    const notFound = isApiError(query.error, 404);
    return (
      <main className={`${styles.detail} ${styles.message}`}>
        <Notice
          tone={notFound ? 'info' : 'danger'}
          title={notFound ? t('item.notFound.title') : problemMessage(t, query.error)}
          actions={
            <ButtonLink href={paths.home} variant="primary">
              {t('item.notFound.action')}
            </ButtonLink>
          }
        >
          {notFound && t('item.notFound.body')}
        </Notice>
      </main>
    );
  }

  const item = query.data;
  const { images } = item;
  const hasArt = Boolean(
    images.backdrop ?? images.backdropSmall ?? images.backdropLarge ?? images.poster,
  );
  const progress = item.progress && !item.progress.completed ? item.progress : undefined;
  const genres = item.genres.join(', ');

  return (
    <SpatialPage defaultFocusKey={item.mediaFiles.some((f) => f.available) ? PLAY_KEY : BACK_KEY}>
      <main className={styles.detail}>
        <Backdrop
          url={imageUrl(images.backdrop)}
          smallUrl={imageUrl(images.backdropSmall)}
          largeUrl={imageUrl(images.backdropLarge)}
          posterUrl={imageUrl(images.poster)}
          accent={item.accent}
        />

        <div className={`${styles.layout} ${hasArt ? '' : styles.noArt}`}>
          <div className={styles.poster} style={itemAccentStyle(item.accent)}>
            {images.poster ? (
              <img
                src={apiUrl(images.poster.url)}
                srcSet={
                  images.posterLarge
                    ? `${apiUrl(images.poster.url)} 400w, ${apiUrl(images.posterLarge.url)} 800w`
                    : undefined
                }
                sizes={images.posterLarge ? POSTER_SIZES : undefined}
                alt=""
                width={400}
                height={600}
                className={styles.posterImage}
              />
            ) : (
              <span className={styles.posterFallback}>
                <PosterFallback />
              </span>
            )}
          </div>

          <div className={styles.info}>
            <div className={styles.heading}>
              <h1 className={styles.title}>{item.title}</h1>
              {item.originalTitle && item.originalTitle !== item.title && (
                <p className={styles.original}>
                  {t('item.originalTitle', { title: item.originalTitle })}
                </p>
              )}
            </div>
            <div className={styles.factLines}>
              <ItemMeta item={item} />
              {genres && <p className={styles.genres}>{genres}</p>}
            </div>
            {item.tagline && <p className={styles.tagline}>{item.tagline}</p>}

            <Actions item={item} onBack={goBack} />

            {progress && (
              <div className={styles.progress}>
                <ProgressBar
                  value={progress.percent}
                  label={t('item.watched', { percent: Math.round(progress.percent) })}
                />
                <span className={styles.progressText}>
                  {t('item.watched', { percent: Math.round(progress.percent) })}
                </span>
              </div>
            )}

            {item.overview && <p className={styles.overview}>{item.overview}</p>}

            <section className={styles.files} aria-labelledby="item-file-heading">
              <h2 id="item-file-heading" className={styles.sectionTitle}>
                {t('item.file.title')}
              </h2>
              {item.mediaFiles.length === 0 ? (
                <Notice tone="warning">{t('item.file.none')}</Notice>
              ) : (
                item.mediaFiles.map((file) => (
                  <FileFacts key={file.id} file={file} t={t} language={i18n.language} />
                ))
              )}
            </section>
          </div>
        </div>
      </main>
    </SpatialPage>
  );
}
