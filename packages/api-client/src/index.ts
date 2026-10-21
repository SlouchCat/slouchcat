import createClient, { type Client } from 'openapi-fetch';
import type { components, paths } from './generated/schema';

export type { components, paths } from './generated/schema';

export type Health = components['schemas']['Health'];
export type SessionInfo = components['schemas']['SessionInfo'];
export type Account = components['schemas']['Account'];
export type Profile = components['schemas']['Profile'];
export type ProfileSummary = components['schemas']['ProfileSummary'];
export type ProfileRequest = components['schemas']['ProfileRequest'];
export type ProfileUpdate = components['schemas']['ProfileUpdate'];
export type Accent = components['schemas']['Accent'];
export type Language = components['schemas']['Language'];
export type Rating = components['schemas']['Rating'];
export type Invitation = components['schemas']['Invitation'];
export type InvitationCreated = components['schemas']['InvitationCreated'];
export type InvitationRequest = components['schemas']['InvitationRequest'];
export type InvitationPreview = components['schemas']['InvitationPreview'];
export type AcceptInvitationRequest = components['schemas']['AcceptInvitationRequest'];
export type Guest = components['schemas']['Guest'];
export type GuestUpdate = components['schemas']['GuestUpdate'];
export type SetupRequest = components['schemas']['SetupRequest'];
export type LoginRequest = components['schemas']['LoginRequest'];
export type HomePage = components['schemas']['HomePage'];
export type HomeRow = components['schemas']['HomeRow'];
export type ItemRef = components['schemas']['ItemRef'];
export type ItemDetail = components['schemas']['ItemDetail'];
export type MediaFileInfo = components['schemas']['MediaFileInfo'];
export type PlaybackInfo = components['schemas']['PlaybackInfo'];
export type PlaybackRequest = components['schemas']['PlaybackRequest'];
export type Quality = components['schemas']['Quality'];
export type QualityOption = components['schemas']['QualityOption'];
export type Track = components['schemas']['Track'];
export type TrackList = components['schemas']['TrackList'];
export type Chapter = components['schemas']['Chapter'];
export type Segment = components['schemas']['Segment'];
export type WatchTime = components['schemas']['WatchTime'];
export type WatchReport = components['schemas']['WatchReport'];
export type PlaybackLimits = components['schemas']['PlaybackLimits'];
export type EffectiveLimits = components['schemas']['EffectiveLimits'];
export type AdminDevice = components['schemas']['AdminDevice'];
export type AdminStream = components['schemas']['AdminStream'];
export type ProgressEvent = components['schemas']['ProgressEvent'];
export type Progress = components['schemas']['Progress'];
export type Library = components['schemas']['Library'];
export type LibrarySummary = components['schemas']['LibrarySummary'];
export type LibraryRequest = components['schemas']['LibraryRequest'];
export type LibraryUpdate = components['schemas']['LibraryUpdate'];
export type AdminPodcast = components['schemas']['AdminPodcast'];
export type PodcastCreate = components['schemas']['PodcastCreate'];
export type PodcastEpisodes = components['schemas']['PodcastEpisodes'];
export type PlaybackPart = components['schemas']['PlaybackPart'];
export type FieldError = components['schemas']['FieldError'];
export type ExtraKind = components['schemas']['ExtraKind'];
export type CastMember = components['schemas']['CastMember'];
export type Person = components['schemas']['Person'];
export type Trailer = components['schemas']['Trailer'];
export type TrailerRequest = components['schemas']['TrailerRequest'];
export type MetadataProvider = components['schemas']['MetadataProvider'];
export type MetadataCandidate = components['schemas']['MetadataCandidate'];
export type ItemMatchRequest = components['schemas']['ItemMatchRequest'];
export type ItemsRatingRequest = components['schemas']['ItemsRatingRequest'];
export type AdminItem = components['schemas']['AdminItem'];
export type AdminItemRating = components['schemas']['AdminItemRating'];
export type AdminMediaFile = components['schemas']['AdminMediaFile'];
export type IssueKind = components['schemas']['IssueKind'];
export type Issue = components['schemas']['Issue'];
export type IssuePage = components['schemas']['IssuePage'];
export type DuplicateGroup = components['schemas']['DuplicateGroup'];
export type MergeRequest = components['schemas']['MergeRequest'];
export type CleanupRequest = components['schemas']['CleanupRequest'];
/** RFC 9457 problem details as returned by every failing JSON endpoint. */
export type Problem = components['schemas']['Problem'];
export type ApiClient = Client<paths>;

/** Same-origin base path of the API (the `servers` entry of api/openapi.yaml). */
export const DEFAULT_BASE_URL = '/api/v1';

/**
 * Header every state-changing request must carry (contract description). Browsers never add a
 * custom header to cross-site form posts, so with the SameSite=Lax cookie this blocks CSRF.
 */
export const CSRF_HEADER = 'X-Requested-With';
export const CSRF_HEADER_VALUE = 'Slouchcat';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);

/**
 * Creates a typed client for the Slouchcat API.
 *
 * A relative `baseUrl` is resolved against the current document location when there is one,
 * so the same client works in the browser and in DOM test environments. `fetch` is looked up
 * on every request, which lets tests replace `globalThis.fetch` after the client was created.
 */
export function createApiClient(baseUrl: string = DEFAULT_BASE_URL): ApiClient {
  const location = (globalThis as { location?: { href: string } }).location;
  return createClient<paths>({
    baseUrl: location ? new URL(baseUrl, location.href).href : baseUrl,
    credentials: 'same-origin',
    fetch: (request) => {
      // Only state-changing requests need the CSRF header. Reads go without it, so they match the
      // responses the server preloads for the start page (Link: rel=preload; as=fetch, P-10).
      if (!SAFE_METHODS.has(request.method)) request.headers.set(CSRF_HEADER, CSRF_HEADER_VALUE);
      return globalThis.fetch(request);
    },
  });
}

/** Narrows an unknown error body to {@link Problem}. */
export function isProblem(value: unknown): value is Problem {
  if (typeof value !== 'object' || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.type === 'string' &&
    typeof candidate.title === 'string' &&
    typeof candidate.status === 'number' &&
    typeof candidate.code === 'string'
  );
}
// Discover and curation (M7).
export type SearchRequest = components['schemas']['SearchRequest'];
export type SearchResult = components['schemas']['SearchResult'];
export type PersonRef = components['schemas']['PersonRef'];
export type GenreRef = components['schemas']['GenreRef'];
export type CollectionRef = components['schemas']['CollectionRef'];
export type Collection = components['schemas']['Collection'];
export type ItemPage = components['schemas']['ItemPage'];
export type Notification = components['schemas']['Notification'];
export type NotificationPage = components['schemas']['NotificationPage'];
export type AdminPick = components['schemas']['AdminPick'];
export type PickRequest = components['schemas']['PickRequest'];
export type PickUpdate = components['schemas']['PickUpdate'];
export type PickOrder = components['schemas']['PickOrder'];
export type AdminItemHit = components['schemas']['AdminItemHit'];
export type AdminCollection = components['schemas']['AdminCollection'];
export type AdminCollectionItem = components['schemas']['AdminCollectionItem'];
export type CollectionRequest = components['schemas']['CollectionRequest'];
export type CollectionItemsRequest = components['schemas']['CollectionItemsRequest'];
export type HomeRowSetting = components['schemas']['HomeRowSetting'];
export type HomeRowKey = components['schemas']['HomeRowKey'];
export type TraktStatus = components['schemas']['TraktStatus'];
