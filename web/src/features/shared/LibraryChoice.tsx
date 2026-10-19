import { useTranslation } from 'react-i18next';
import { CheckboxGroup, RadioGroup } from '@/ui/aria';
import type { LibraryOption } from './useLibraryOptions';

export interface LibrarySelection {
  /** Every library the account may give out. */
  all: boolean;
  /** Library ids as strings, used when `all` is false. */
  ids: string[];
}

interface LibraryChoiceProps {
  label: string;
  value: LibrarySelection;
  onChange: (value: LibrarySelection) => void;
  options: readonly LibraryOption[];
}

/**
 * "All libraries" or a checklist of chosen ones (profiles, invitations, guests). The caller has a
 * known list of options (`useLibraryOptions().isReady`); with none to choose from only "all" is
 * offered, and a stored restriction is shown as it is, never widened.
 */
export function LibraryChoice({ label, value, onChange, options }: LibraryChoiceProps) {
  const { t } = useTranslation();
  return (
    <>
      <RadioGroup
        name="allLibraries"
        label={label}
        value={value.all ? 'all' : 'some'}
        onChange={(mode) => onChange({ ...value, all: mode === 'all' })}
        options={[
          { value: 'all', label: t('libraries.modeAll') },
          { value: 'some', label: t('libraries.modeSome'), isDisabled: options.length === 0 },
        ]}
      />
      {!value.all && (
        <CheckboxGroup
          name="libraryIds"
          label={t('libraries.choose')}
          value={value.ids}
          onChange={(ids) => onChange({ ...value, ids })}
          options={options}
          validate={(ids) => (ids.length > 0 ? null : t('libraries.needOne'))}
        />
      )}
    </>
  );
}
