// Sample data shared by the stories (*.stories.tsx). Artwork lives in web/.storybook/public
// (make-fixtures.sh); the accents are what the image pipeline would derive from it (ItemRef.accent).
export const art = {
  posterDusk: 'poster-dusk.jpg',
  posterEmber: 'poster-ember.jpg',
  posterMoss: 'poster-moss.jpg',
  backdropDusk: 'backdrop-dusk.jpg',
  /** A title logo with transparent ground (concept 10, 7.5): inline, so no fixture file is needed. */
  logoEmber:
    "data:image/svg+xml;utf8,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 600 160'%3E%3Ctext x='300' y='110' text-anchor='middle' font-family='Georgia, serif' font-size='96' font-weight='700' fill='%23f2c27a' stroke='%231a0c08' stroke-width='4'%3EGLUT%3C/text%3E%3C/svg%3E",
} as const;

export const accents = {
  dusk: '#2f6fb3',
  ember: '#b5482a',
  moss: '#3d7d4f',
} as const;

export const titles = [
  { id: 'dusk', title: 'Die Stunde der Dämmerung', year: 2023, poster: art.posterDusk },
  { id: 'ember', title: 'Glut', year: 2019, poster: art.posterEmber },
  { id: 'moss', title: 'Moos und Stein', year: 2021, poster: art.posterMoss },
] as const;
