import { describe, expect, it } from 'vitest';
import {
  folderMetadataDraft,
  folderMetadataRequest,
  folderMetadataValues,
  hasOwnMetadata,
} from './FolderMetadataFields';

// The draft of «Sprachen und Bilder eines Ordners» and what it sends (concept 24 3.6): a new
// folder sends only its own values; the form «Bearbeiten» sends a folder's whole state, `null`
// for what follows the default (saveLibrary).

const own = { textLangs: ['ja', 'en'], images: { langs: { poster: ['ja'] }, kinds: [] } };
const plain = { textLangs: null, images: null };

describe('folder metadata draft', () => {
  it('starts from the folder: «Wie Standard» for null, «Eigene» with the list for a value', () => {
    expect(folderMetadataDraft(plain)).toEqual({ ownLangs: false, ownImages: false });
    expect(folderMetadataDraft()).toEqual({ ownLangs: false, ownImages: false });
    expect(folderMetadataDraft(own)).toEqual({ ownLangs: true, ownImages: true, ...own });
    expect(hasOwnMetadata(folderMetadataDraft(plain))).toBe(false);
    expect(hasOwnMetadata(folderMetadataDraft(own))).toBe(true);
  });

  it('a new folder sends only what is its own; an empty kind list is its own too', () => {
    expect(folderMetadataRequest(undefined)).toEqual({});
    expect(folderMetadataRequest({ ownLangs: false, textLangs: ['fr'], ownImages: false })).toEqual(
      {},
    );
    expect(
      folderMetadataRequest({
        ownLangs: true,
        textLangs: ['fr'],
        ownImages: true,
        images: { langs: {}, kinds: [] },
      }),
    ).toEqual({ textLangs: ['fr'], images: { langs: {}, kinds: [] } });
  });

  it('a stored folder: its whole state, null back to the default, the value when own', () => {
    expect(folderMetadataValues(folderMetadataDraft(own))).toEqual(own);
    expect(folderMetadataValues(folderMetadataDraft(plain))).toEqual(plain);
    // Back to the default: the kept list goes with the save, as null.
    expect(
      folderMetadataValues({ ...folderMetadataDraft(own), ownLangs: false, ownImages: false }),
    ).toEqual(plain);
    expect(
      folderMetadataValues({ ...folderMetadataDraft(plain), ownLangs: true, textLangs: ['de'] }),
    ).toEqual({ textLangs: ['de'], images: null });
  });
});
