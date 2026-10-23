import { useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { formatCountdown, useCountdown } from '@/lib/useCountdown';
import { Notice, VisuallyHidden } from '@/ui';
import { AriaButton, Form, FormDialog, PinField } from '@/ui/aria';
import { openOfflineProfile } from './offlineProfiles';
import { lockRemainingMs, pinRequirement } from './pin';
import type { StoredProfile } from './types';
import styles from './OfflinePages.module.css';

interface OfflinePinDialogProps {
  profile: StoredProfile;
  onClose: () => void;
  /** The PIN matched: the profile may open the offline library. */
  onOpened: () => void;
}

const OFFLINE_DIGITS = { min: 6, max: 12 };

/**
 * PIN of a protected profile in the offline library (AN-19): checked against the separate offline
 * PIN or the device's verifier of the normal PIN. After the third wrong PIN the device pauses
 * 30 s, doubling up to 15 min. A profile without a verifier here cannot be opened offline.
 */
export function OfflinePinDialog({ profile, onClose, onOpened }: OfflinePinDialogProps) {
  const { t } = useTranslation();
  const requirement = pinRequirement(profile);
  const offlinePin = requirement === 'offlinePin';
  const [pin, setPin] = useState('');
  const [checking, setChecking] = useState(false);
  const [invalid, setInvalid] = useState(false);
  const [lock, setLock] = useState(() => {
    const ms = lockRemainingMs(profile.lock, Date.now());
    return { seconds: ms > 0 ? Math.ceil(ms / 1000) : null, run: 0 };
  });
  const remaining = useCountdown(lock.seconds, lock.run);
  const locked = remaining > 0;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (locked || checking) return;
    setChecking(true);
    setInvalid(false);
    void openOfflineProfile(profile.id, pin).then((result) => {
      setChecking(false);
      setPin('');
      if (result.ok) {
        onOpened();
        return;
      }
      if (result.reason === 'unavailable') return;
      setInvalid(result.reason === 'invalid');
      if (result.retryInMs > 0) {
        setLock((current) => ({
          seconds: Math.ceil(result.retryInMs / 1000),
          run: current.run + 1,
        }));
      }
    });
  };

  let message: ReactNode = null;
  if (locked) {
    message = (
      <Notice tone="warning">
        <VisuallyHidden>{t('profiles.pin.lockedFor', { seconds: lock.seconds })}</VisuallyHidden>
        <span aria-hidden="true">
          {t('profiles.pin.locked')}{' '}
          <span className={styles.time}>{t('profiles.pin.time', formatCountdown(remaining))}</span>
        </span>
      </Notice>
    );
  } else if (invalid) {
    message = <Notice tone="danger">{t('profiles.pin.invalid')}</Notice>;
  }

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('profiles.pin.title', { name: profile.snapshot.name })}
      description={
        requirement === 'unavailable'
          ? undefined
          : offlinePin
            ? t('offline.pin.introOffline')
            : t('profiles.pin.intro')
      }
    >
      {requirement === 'unavailable' ? (
        <div className={styles.form}>
          <Notice tone="warning" title={t('offline.pin.unavailable.title')}>
            {t('offline.pin.unavailable.body')}
          </Notice>
          <div className={styles.formActions}>
            <AriaButton variant="primary" onPress={onClose} autoFocus>
              {t('common.close')}
            </AriaButton>
          </div>
        </div>
      ) : (
        <Form className={styles.form} onSubmit={onSubmit}>
          {message}
          <PinField
            name="pin"
            label={offlinePin ? t('offline.pin.offlineLabel') : t('profiles.pin.label')}
            description={offlinePin ? t('offline.pin.offlineHint') : t('profiles.pin.hint')}
            invalidMessage={
              offlinePin ? t('offline.pin.offlineFormat') : t('errors.field.pin_format')
            }
            digits={offlinePin ? OFFLINE_DIGITS : undefined}
            value={pin}
            onChange={setPin}
            autoFocus
          />
          <div className={styles.formActions}>
            <AriaButton variant="ghost" onPress={onClose}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton type="submit" variant="primary" isDisabled={locked} isPending={checking}>
              {checking ? t('profiles.pin.submitting') : t('profiles.pin.submit')}
            </AriaButton>
          </div>
        </Form>
      )}
    </FormDialog>
  );
}
