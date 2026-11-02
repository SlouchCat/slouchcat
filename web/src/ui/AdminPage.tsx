import { useId, useState, type ReactNode, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { AdminDensityScope } from './adminDensity';
import styles from './AdminPage.module.css';
import { Icon } from './Icon';
import { Page } from './Page';
import { StatusDot } from './StatusDot';
import { Text } from './Text';

interface AdminPageProps {
  /** The h1 of the page. */
  title: string;
  /** The page's intro sentence; shown only behind the help button (S-2). */
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
 * the AdminDensityScope, so the dialogs of the page share its density (ADR 0020).
 */
export function AdminPage({
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
    <Page>
      <AdminDensityScope>
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
              {liveSeconds !== undefined && (
                <span className={styles.live}>
                  <StatusDot state="success" />
                  {t('ui.adminPage.live', { count: liveSeconds })}
                </span>
              )}
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
      </AdminDensityScope>
    </Page>
  );
}
