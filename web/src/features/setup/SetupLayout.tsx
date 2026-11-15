import type { SessionInfo, SetupProgress, SetupStep } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useEffect, useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { sessionQuery } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useWideScreen } from '@/lib/useWideScreen';
import { Logo, Notice, Text } from '@/ui';
import {
  AdminDensityScope,
  AdminSideNavContent,
  AdminSideNavMenu,
  AriaButton,
  type AdminSideNavEntry,
} from '@/ui/aria';
import { LanguageButton } from '../auth/LoginPage';
import {
  deferSetup,
  previousStep,
  PUBLIC_STEPS,
  REQUIRED_STEPS,
  SETUP_STEPS,
  setupPath,
  setupProgressQuery,
  STEP_ICONS,
  stepState,
} from './setupState';
import styles from './SetupLayout.module.css';

export interface SetupFooter {
  /** «Weiter» (or «Einrichtung abschliessen» in the summary); the step saves, then goes on. */
  onNext: () => void;
  nextLabel?: string;
  isPending?: boolean;
  isDisabled?: boolean;
  /** «Überspringen» of a voluntary step; the layout leaves it out for the required ones. */
  onSkip?: () => void;
}

interface SetupLayoutProps {
  step: SetupStep;
  /** The intro below the heading. */
  intro?: ReactNode;
  /** The buttons at the foot; none on the finish page. */
  footer?: SetupFooter;
  /** A message of the step's last save, above the footer. */
  error?: ReactNode;
  children: ReactNode;
}

/** The step list's entries: every step with its state; before the account only 1 and 2 lead anywhere. */
function useStepEntries(
  current: SetupStep,
  session: SessionInfo | null | undefined,
  progress: SetupProgress | undefined,
): AdminSideNavEntry[] {
  const { t } = useTranslation();
  return SETUP_STEPS.map((step, index) => {
    const label = t(`setup.steps.${step}.nav`);
    const state = !session
      ? PUBLIC_STEPS.indexOf(step) < PUBLIC_STEPS.indexOf(current) && index < 2
        ? 'done'
        : 'open'
      : PUBLIC_STEPS.includes(step)
        ? 'done'
        : stepState(progress, step);
    // Before the account a later step leads to the account step, the next one that is needed.
    const href = session || PUBLIC_STEPS.includes(step) ? setupPath(step) : setupPath('account');
    return {
      href,
      label,
      icon: state === 'done' ? 'check' : state === 'skipped' ? 'next' : STEP_ICONS[step],
      current: step === current,
      accessibleLabel: t(`setup.nav.state.${state}`, { position: index + 1, name: label }),
    };
  });
}

/**
 * The frame of every step of the guided wizard (concept 22, 3.6): the step list on the left as in
 * the administration (the same component; on a phone the same sheet), the step's heading, which
 * takes the focus when the step opens, its content, and the foot with «Zurück», «Überspringen»
 * (voluntary steps only) and «Weiter». Before the account the language button stands at the top;
 * after it the list ends with «Später fortsetzen» (F-11, 3.4).
 */
export function SetupLayout({ step, intro, footer, error, children }: SetupLayoutProps) {
  const { t } = useTranslation();
  const title = t(`setup.steps.${step}.title`);
  useDocumentTitle(t('setup.documentTitle', { step: title }));
  const navigate = useNavigate();
  const wide = useWideScreen();
  const heading = useRef<HTMLHeadingElement>(null);
  const session = useQuery(sessionQuery).data;
  const progress = useQuery({ ...setupProgressQuery, enabled: Boolean(session) }).data;
  const entries = useStepEntries(step, session, progress);
  const position = SETUP_STEPS.indexOf(step) + 1;
  const back = previousStep(step);
  // The account cannot be gone back to once it exists, nor any step before it.
  const canGoBack = back !== undefined && !(session && back === 'account');
  const nav = {
    label: t('setup.nav.label'),
    top: [],
    groups: [{ label: t('setup.nav.group'), icon: 'list' as const, entries }],
  };

  // A new step is new content: keyboard and screen reader users land on its heading (3.6).
  useEffect(() => {
    heading.current?.focus();
  }, [step]);

  const later = () => {
    if (!session) return;
    deferSetup(session);
    void navigate({ to: paths.home });
  };

  return (
    <main className={styles.page}>
      <AdminDensityScope>
        <div className={styles.frame}>
          {wide && (
            <aside className={styles.rail}>
              <Logo name={t('app.name')} />
              <AdminSideNavContent {...nav} className={styles.steps} />
              {session && (
                <AriaButton variant="ghost" icon="next" onPress={later}>
                  {t('setup.actions.later')}
                </AriaButton>
              )}
            </aside>
          )}
          <div className={styles.content}>
            {!wide && (
              <div className={styles.top}>
                <AdminSideNavMenu
                  {...nav}
                  buttonLabel={t('setup.nav.label')}
                  pageLabel={title}
                  menuLabel={t('setup.nav.menu')}
                  closeLabel={t('setup.nav.close')}
                />
              </div>
            )}
            {!session && (
              <div className={styles.language}>
                <LanguageButton />
              </div>
            )}
            <header className={styles.header}>
              <Text tone="secondary" variant="caption">
                {t('setup.position', { position, count: SETUP_STEPS.length })}
              </Text>
              <h1 ref={heading} tabIndex={-1} className={styles.title}>
                {title}
              </h1>
              {intro && <div className={styles.intro}>{intro}</div>}
            </header>
            <div className={styles.body}>{children}</div>
            {error && (
              <Notice tone="danger" live={false}>
                {error}
              </Notice>
            )}
            {footer && (
              <div className={styles.footer}>
                {canGoBack && (
                  <AriaButton
                    variant="ghost"
                    icon="back"
                    onPress={() => void navigate({ to: setupPath(back) })}
                  >
                    {t('setup.actions.back')}
                  </AriaButton>
                )}
                <span className={styles.spacer} />
                {footer.onSkip && !REQUIRED_STEPS.includes(step) && (
                  <AriaButton variant="secondary" onPress={footer.onSkip}>
                    {t('setup.actions.skip')}
                  </AriaButton>
                )}
                <AriaButton
                  variant="primary"
                  isPending={footer.isPending}
                  isDisabled={footer.isDisabled}
                  onPress={footer.onNext}
                >
                  {footer.nextLabel ?? t('setup.actions.next')}
                </AriaButton>
              </div>
            )}
            {!wide && session && (
              <div className={styles.laterNarrow}>
                <AriaButton variant="ghost" icon="next" onPress={later}>
                  {t('setup.actions.later')}
                </AriaButton>
              </div>
            )}
          </div>
        </div>
      </AdminDensityScope>
    </main>
  );
}
