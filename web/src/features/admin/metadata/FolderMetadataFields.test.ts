import { describe, expect, it } from 'vitest';
import {
  folderMetadataDraft,
  folderMetadataRequest,
  folderMetadataUpdate,
  hasOwnMetadata,
} from './FolderMetadataFields';

// The draft of «Sprachen und Bilder eines Ordners» and what it sends (concept 24 3.6): a new
// folder sends only its own values; a changed folder has three states per field.

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

  it('a changed folder: left out when unchanged, null back to the default, the value when own', () => {
    expect(folderMetadataUpdate(folderMetadataDraft(own), own)).toEqual({});
    // Back to the default: the kept list goes with the save, as null.
    expect(
      folderMetadataUpdate({ ...folderMetadataDraft(own), ownLangs: false, ownImages: false }, own),
    ).toEqual({ textLangs: null, images: null });
    expect(
      folderMetadataUpdate(
        { ...folderMetadataDraft(plain), ownLangs: true, textLangs: ['de'] },
        plain,
      ),
    ).toEqual({ textLangs: ['de'] });
    // Switched back and forth before the save: nothing changed.
    expect(folderMetadataUpdate({ ...folderMetadataDraft(own), ownLangs: true }, own)).toEqual({});
  });
});
