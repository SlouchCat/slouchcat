import type { Announcement } from '@slouchcat/api-client';
import { useRef, type ReactNode, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { AnnouncementContent } from '@/features/notifications/AnnouncementContent';
import { formatDateTime } from '@/lib/format';
import { Chip, Notice, ProgressBar } from '@/ui';
import { AriaButton, BandPanel, CheckboxGroup, Form, TextField } from '@/ui/aria';
import styles from './Announcements.module.css';

/** Longest title and text the server takes (AnnouncementRequest). */
export const MAX_TITLE = 120;
export const MAX_BODY = 4000;

/** What is typed for an announcement: the draft of a saved one (useDrafts, S-17). */
export interface DraftText {
  title: string;
  body: string;
}

/** The new announcement while it is not saved yet: its text and «Sofort veröffentlichen». */
export interface NewAnnouncement extends DraftText {
  publish: boolean;
}

/**
 * «So sehen es die Profile»: the content of the profiles' announcement dialog without its buttons,
 * with what is typed right now. An unpublished one shows today's date, the day it would go out.
 */
function Preview({ text, publishedAt }: { text: DraftText; publishedAt?: string }) {
  const { t } = useTranslation();
  return (
    <div className={styles.detail}>
      <h3 className={styles.previewTitle}>{t('admin.announcements.preview.title')}</h3>
      <div className={styles.stage}>
        <div className={styles.card}>
          <AnnouncementContent
            title={text.title.trim() || t('admin.announcements.preview.noTitle')}
            body={text.body.trim() ? text.body : t('admin.announcements.preview.noBody')}
            publishedAt={publishedAt ?? new Date().toISOString()}
          />
        </div>
      </div>
    </div>
  );
}

/** Created, published and «Gelesen» of a saved announcement (ANK-2). */
function Facts({ announcement }: { announcement: Announcement }) {
  const { t, i18n } = useTranslation();
  const { acknowledgedCount, profileCount, publishedAt } = announcement;
  const read = t('admin.announcements.acknowledged', {
    acknowledged: acknowledgedCount,
    total: profileCount,
  });
  return (
    <dl className={styles.facts}>
      <div>
        <dt>{t('admin.announcements.facts.created')}</dt>
        <dd>
          <time dateTime={announcement.createdAt}>
            {formatDateTime(announcement.createdAt, i18n.language)}
          </time>
        </dd>
      </div>
      {publishedAt && (
        <div>
          <dt>{t('admin.announcements.facts.published')}</dt>
          <dd>
            <time dateTime={publishedAt}>{formatDateTime(publishedAt, i18n.language)}</time>
          </dd>
        </div>
      )}
      <div>
        <dt>{t('admin.announcements.facts.acknowledged')}</dt>
        <dd>
          {publishedAt ? (
            // The bar for the eye, the same in words beside it.
            <span className={styles.read}>
              <span className={styles.bar}>
                <ProgressBar
                  value={profileCount > 0 ? (acknowledgedCount / profileCount) * 100 : 0}
                  label={t('admin.announcements.facts.acknowledged')}
                  size="regular"
                />
              </span>
              <span className={styles.numbers}>{read}</span>
            </span>
          ) : (
            <span className={styles.muted}>{t('admin.announcements.notSent')}</span>
          )}
        </dd>
      </div>
    </dl>
  );
}

interface AnnouncementFormProps {
  /** Spoken name of the form («Neue Ankündigung», «Entwurf bearbeiten»). */
  label: string;
  text: DraftText;
  onChange: (text: DraftText) => void;
  /** The last failed save of this form: a field's message under the field, else above. */
  error: unknown;
  onSubmit: () => void;
  formRef?: Ref<HTMLFormElement>;
  titleRef?: Ref<HTMLInputElement>;
  /** The fields take no input (the text is on its way out). */
  readOnly?: boolean;
  /** Between the fields and the buttons. */
  extra?: ReactNode;
  actions: ReactNode;
}

/** Title (with its length) and text, both required; Enter in the title saves (ANK-1, ANK-3). */
function AnnouncementForm({
  label,
  text,
  onChange,
  error,
  onSubmit,
  formRef,
  titleRef,
  readOnly = false,
  extra,
  actions,
}: AnnouncementFormProps) {
  const { t } = useTranslation();
  const errors = error ? formErrors(t, error) : null;
  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));
  return (
    <Form
      ref={formRef}
      aria-label={label}
      validationErrors={errors?.fields}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <div className={styles.form}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="title"
          label={t('admin.announcements.form.titleLabel')}
          description={t('admin.announcements.form.titleCount', {
            length: text.title.length,
            max: MAX_TITLE,
          })}
          value={text.title}
          onChange={(title) => onChange({ ...text, title })}
          inputRef={titleRef}
          maxLength={MAX_TITLE}
          isRequired
          isReadOnly={readOnly}
          validate={required}
          autoComplete="off"
        />
        <TextField
          name="body"
          label={t('admin.announcements.form.body')}
          description={t('admin.announcements.form.bodyHint', { max: MAX_BODY })}
          value={text.body}
          onChange={(body) => onChange({ ...text, body })}
          rows={8}
          maxLength={MAX_BODY}
          isRequired
          isReadOnly={readOnly}
          validate={required}
        />
        {extra}
        <div className={styles.actions}>{actions}</div>
      </div>
    </Form>
  );
}

interface NewDetailProps {
  value: NewAnnouncement;
  onChange: (value: NewAnnouncement) => void;
  saving: boolean;
  error: unknown;
  onSave: () => void;
  onCancel: () => void;
  titleRef: Ref<HTMLInputElement>;
}

/**
 * «Neue Ankündigung» in the detail (ANK-1): it may go out at once, else it becomes a draft. What
 * is typed while a draft is being saved stays (the page carries it over to the saved draft); one
 * that goes out at once takes no more input meanwhile.
 */
export function NewDetail({
  value,
  onChange,
  saving,
  error,
  onSave,
  onCancel,
  titleRef,
}: NewDetailProps) {
  const { t } = useTranslation();
  return (
    <BandPanel title={t('admin.announcements.form.title')} icon="megaphone">
      <div className={styles.detail}>
        <AnnouncementForm
          label={t('admin.announcements.form.title')}
          text={value}
          onChange={(text) => onChange({ ...value, ...text })}
          error={error}
          onSubmit={onSave}
          titleRef={titleRef}
          readOnly={saving && value.publish}
          extra={
            <CheckboxGroup
              name="publish"
              label={t('admin.announcements.form.publishLabel')}
              description={t('admin.announcements.publish.reach')}
              value={value.publish ? ['publish'] : []}
              onChange={(checked) => onChange({ ...value, publish: checked.includes('publish') })}
              isDisabled={saving}
              options={[{ value: 'publish', label: t('admin.announcements.form.publishNow') }]}
            />
          }
          actions={
            <>
              <AriaButton type="submit" variant="primary" isPending={saving}>
                {saving
                  ? t('common.saving')
                  : value.publish
                    ? t('admin.announcements.publish.confirm')
                    : t('admin.announcements.form.save')}
              </AriaButton>
              <AriaButton variant="ghost" isDisabled={saving} onPress={onCancel}>
                {t('common.cancel')}
              </AriaButton>
            </>
          }
        />
        <Preview text={value} />
      </div>
    </BandPanel>
  );
}

interface DraftDetailProps {
  announcement: Announcement;
  /** What is typed: the draft, else the saved text. */
  text: DraftText;
  dirty: boolean;
  onChange: (text: DraftText) => void;
  saving: boolean;
  /** It is being published: the fields take no input, «Veröffentlichen» waits. */
  publishing: boolean;
  error: unknown;
  onSave: () => void;
  onDiscard: () => void;
  /** Called only when title and text are valid; otherwise the form says what is missing. */
  onPublish: () => void;
  onDelete: () => void;
  titleRef: Ref<HTMLInputElement>;
  saveRef: Ref<HTMLButtonElement>;
}

/**
 * A draft (ANK-3): changed in place, saved, published after a question (what is typed goes out)
 * or deleted after one.
 */
export function DraftDetail({
  announcement,
  text,
  dirty,
  onChange,
  saving,
  publishing,
  error,
  onSave,
  onDiscard,
  onPublish,
  onDelete,
  titleRef,
  saveRef,
}: DraftDetailProps) {
  const { t } = useTranslation();
  const formRef = useRef<HTMLFormElement>(null);
  const name = { title: announcement.title };
  // An empty title or text is not published: instead of the question, the form shows what is
  // missing and the first field without a valid value takes the focus.
  const publishIfValid = () => {
    const form = formRef.current;
    if (form && !form.reportValidity()) {
      const invalid = [...form.elements].find(
        (element): element is HTMLInputElement | HTMLTextAreaElement =>
          (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) &&
          !element.validity.valid,
      );
      invalid?.focus();
      return;
    }
    onPublish();
  };
  return (
    <BandPanel
      title={announcement.title}
      icon="megaphone"
      meta={<Chip tone="outline">{t('admin.announcements.draft')}</Chip>}
    >
      <div className={styles.detail}>
        <Facts announcement={announcement} />
        <AnnouncementForm
          label={t('admin.announcements.form.editTitle')}
          text={text}
          onChange={onChange}
          error={error}
          onSubmit={onSave}
          formRef={formRef}
          titleRef={titleRef}
          readOnly={publishing}
          extra={
            <p className={styles.muted}>
              {t('admin.announcements.publish.later', {
                reach: t('admin.announcements.publish.reach'),
              })}
            </p>
          }
          actions={
            <>
              <AriaButton
                ref={saveRef}
                type="submit"
                variant="primary"
                isPending={saving}
                isDisabled={publishing}
              >
                {saving ? t('common.saving') : t('admin.announcements.form.save')}
              </AriaButton>
              {dirty && (
                <AriaButton variant="ghost" isDisabled={publishing} onPress={onDiscard}>
                  {t('admin.announcements.discard.action')}
                </AriaButton>
              )}
              <AriaButton
                variant="secondary"
                aria-label={t('admin.announcements.publish.label', name)}
                isPending={publishing}
                onPress={publishIfValid}
              >
                {t('admin.announcements.publish.action')}
              </AriaButton>
              <AriaButton
                variant="danger"
                icon="trash"
                aria-label={t('admin.announcements.delete.label', name)}
                isDisabled={publishing}
                onPress={onDelete}
              >
                {t('admin.announcements.delete.action')}
              </AriaButton>
            </>
          }
        />
        <Preview text={text} />
      </div>
    </BandPanel>
  );
}

/** A published announcement (ANK-2, ANK-3): as it was sent, how many read it, and «Löschen». */
export function PublishedDetail({
  announcement,
  onDelete,
}: {
  announcement: Announcement;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  return (
    <BandPanel title={announcement.title} icon="megaphone">
      <div className={styles.detail}>
        <Facts announcement={announcement} />
        <div className={styles.actions}>
          <AriaButton
            variant="danger"
            icon="trash"
            aria-label={t('admin.announcements.delete.label', { title: announcement.title })}
            onPress={onDelete}
          >
            {t('admin.announcements.delete.action')}
          </AriaButton>
        </div>
        <Preview text={announcement} publishedAt={announcement.publishedAt} />
      </div>
    </BandPanel>
  );
}
