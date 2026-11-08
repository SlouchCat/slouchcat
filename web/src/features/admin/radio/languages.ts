/** ISO 639-1 codes from a field («de, fr»), each once, in lower case; anything else is dropped. */
export const languageCodes = (text: string) => [
  ...new Set(
    text
      .split(/[\s,;]+/)
      .map((code) => code.trim().toLowerCase())
      .filter((code) => /^[a-z]{2}$/.test(code)),
  ),
];
