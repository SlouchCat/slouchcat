// hls.js ships one declaration file for the full and the light build; the light entry point
// (no subtitles, EME or alternate audio, see ADR 0004) has the same API.
declare module 'hls.js/light' {
  export * from 'hls.js';
  export { default } from 'hls.js';
}
