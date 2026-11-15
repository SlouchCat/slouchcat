import { useId, useState, type ReactNode, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { AdminDensityScope } from './adminDensity';
import { useAdminFrame, type AdminFrameValue } from './adminFrame';
import styles from './AdminPage.module.css';
import { AreaTabs } from './AreaTabs';
import { Icon } from './Icon';
import { Page } from './Page';
import { StatusDot } from './StatusDot';
import { Text } from './Text';

interface AdminPageProps {
  /** The h1 of the page. */
  title: string;
  /**
   * The page's intro sentence: inside the administration's frame open below the title (build plan
   * 2.6), elsewhere behind the help button (S-2).
   */
  help?: string;
  /**
   * The page polls every so many seconds: «Live, alle 5 Sekunden» next to the title. A page that
   * polls only under a condition (a scan) passes it only while it polls.
   */
  liveSeconds?: number;
  /** The page's main action(s), at the end of the header line. */
  actions?: ReactNode;
  /** Counters right below the header (StatusStrip, S-4). */
  status?: ReactNode;
  /**
   * The h1, focusable by script (`tabIndex={-1}`): a page moves the focus there when the row that
   * had it is gone (ALL-2).
   */
  titleRef?: Ref<HTMLHeadingElement>;
  children: ReactNode;
}

/**
 * The frame of an administration page (concept 12, S-2 to S-4): a header in one line with the h1,
 * a small help button that shows the intro sentence, the live indicator and the main actions;
 * then the status strip and the content, the sections spaced by `--sc-section-gap`. It also opens
 * the AdminDensityScope, so the dialogs of the page share its density (ADR 0020). Without the
 * administration's shell (`data-shell="admin"`, the `/settings/*` pages) the scope has no effect:
 * density.css applies the compact values only under that shell.
 *
 * Inside the administration's frame (AdminFrame, set by the admin layout; build plan 2.5 and 2.6)
 * the header is the mockup's: the area's tabs above it, the line «MEDIEN / BIBLIOTHEKEN» in the
 * accent over a large title, the intro sentence open below it, the actions at the end; the help
 * button is not needed there.
 */
export function AdminPage(props: AdminPageProps) {
  const frame = useAdminFrame();
  return (
    <Page>
      <AdminDensityScope>
        {frame ? <FramedPage {...props} frame={frame} /> : <PlainPage {...props} />}
      </AdminDensityScope>
    </Page>
  );
}

function Live({ seconds }: { seconds: number }) {
  const { t } = useTranslation();
  return (
    <span className={styles.live}>
      <StatusDot state="success" />
      {t('ui.adminPage.live', { count: seconds })}
    </span>
  );
}

/** Outside the administration's frame: the header as it was (concept 12, S-2). */
function PlainPage({
  title,
  help,
  liveSeconds,
  actions,
  status,
  titleRef,
  children,
}: AdminPageProps) {
  const { t } = useTranslation();
  const helpId = useId();
  const [helpOpen, setHelpOpen] = useState(false);
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.line}>
          <h1 ref={titleRef} tabIndex={-1} className={styles.title}>
            <Text as="span" variant="pageTitle">
              {title}
            </Text>
          </h1>
          {help && (
            <button
              type="button"
              className={styles.help}
              aria-label={t('ui.adminPage.help')}
              aria-expanded={helpOpen}
              aria-controls={helpId}
              onClick={() => setHelpOpen((open) => !open)}
            >
              <Icon name="info" size="sm" />
            </button>
          )}
          {liveSeconds !== undefined && <Live seconds={liveSeconds} />}
          {actions && <div className={styles.actions}>{actions}</div>}
        </div>
        {help && (
          <p id={helpId} className={styles.intro} hidden={!helpOpen}>
            {help}
          </p>
        )}
      </header>
      {status}
      {children}
    </div>
  );
}

/** Inside the administration's frame (build plan 2.5, 2.6). */
function FramedPage({
  title,
  help,
  liveSeconds,
  actions,
  status,
  titleRef,
  children,
  frame,
}: AdminPageProps & { frame: AdminFrameValue }) {
  return (
    <div className={styles.page}>
      {frame.tabs && <AreaTabs {...frame.tabs} />}
      <header className={styles.framed}>
        <div className={styles.heading}>
          <p className={styles.eyebrow}>{frame.eyebrow}</p>
          <div className={styles.line}>
            <h1 ref={titleRef} tabIndex={-1} className={styles.framedTitle}>
              {title}
            </h1>
            {liveSeconds !== undefined && <Live seconds={liveSeconds} />}
          </div>
          {help && <p className={styles.framedIntro}>{help}</p>}
        </div>
        {actions && <div className={styles.framedActions}>{actions}</div>}
      </header>
      {status}
      {children}
    </div>
  );
}
