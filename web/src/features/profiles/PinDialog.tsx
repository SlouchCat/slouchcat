import type { ProfileSummary } from '@slouchcat/api-client';
import { useState, type FormEvent } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage, retryAfterSeconds } from '@/api/errors';
import { useSelectProfile } from '@/app/session';
import { offlineSupported } from '@/features/offline/support';
import { formatCountdown, useCountdown } from '@/lib/useCountdown';
import { Notice, VisuallyHidden } from '@/ui';
import { AriaButton, FormDialog, PinField } from '@/ui/aria';
import styles from './PinDialog.module.css';

interface PinDialogProps {
  profile: ProfileSummary;
  onClose: () => void;
  /** The profile is active now (the session carries it). */
  onSelected: () => void;
}

/**
 * Asks for the PIN of a protected profile (K-4, AN-11). A wrong PIN is reported; after the third
 * wrong one the server locks the profile for this device and the dialog counts the pause down
 * with the submit button off until it is over.
 */
export function PinDialog({ profile, onClose, onSelected }: PinDialogProps) {
  const { t } = useTranslation();
  const select = useSelectProfile();
  const [pin, setPin] = useState('');

  // Any 429 with a named pause (a locked profile, or the attempt limit) counts down and holds the
  // button; the pause is the one the server named.
  const error = select.error;
  const lockSeconds = isApiError(error, 429) ? retryAfterSeconds(error) : null;
  const remaining = useCountdown(lockSeconds, select.failures);
  const locked = remaining > 0;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (locked || select.isPending) return;
    const entered = pin;
    void select.select({ id: profile.id, pin: entered }).then((outcome) => {
      if (outcome.ok) {
        // AN-19: a correct PIN online lets this device open the profile offline later.
        if (offlineSupported()) {
          void import('@/features/offline/offlineProfiles')
            .then(({ rememberPinVerifier }) => rememberPinVerifier(profile.id, entered))
            .catch(() => undefined);
        }
        onSelected();
      } else {
        setPin('');
      }
    });
  };

  let message: React.ReactNode = null;
  if (locked) {
    message = (
      <Notice tone="warning">
        {/* Read out once; the ticking time below is hidden from screen readers. */}
        <VisuallyHidden>{t('profiles.pin.lockedFor', { seconds: lockSeconds })}</VisuallyHidden>
        <span aria-hidden="true">
          {t('profiles.pin.locked')}{' '}
          <span className={styles.time}>{t('profiles.pin.time', formatCountdown(remaining))}</span>
        </span>
      </Notice>
    );
  } else if (error && lockSeconds === null) {
    message = (
      <Notice tone="danger">
        {isApiError(error, 403) && error.code === 'pin_invalid'
          ? t('profiles.pin.invalid')
          : problemMessage(t, error)}
      </Notice>
    );
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('profiles.pin.title', { name: profile.name })}
      description={t('profiles.pin.intro')}
    >
      <Form className={styles.form} onSubmit={onSubmit}>
        {message}
        <PinField
          name="pin"
          label={t('profiles.pin.label')}
          description={t('profiles.pin.hint')}
          invalidMessage={t('errors.field.pin_format')}
          value={pin}
          onChange={setPin}
          autoFocus
        />
        <div className={styles.actions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            isDisabled={locked}
            isPending={select.isPending}
          >
            {select.isPending ? t('profiles.pin.submitting') : t('profiles.pin.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
