// Sample data shared by the stories (*.stories.tsx). Artwork lives in web/.storybook/public
// (make-fixtures.sh); the accents are what the image pipeline would derive from it (ItemRef.accent).
export const art = {
  posterDusk: 'poster-dusk.jpg',
  posterEmber: 'poster-ember.jpg',
  posterMoss: 'poster-moss.jpg',
  backdropDusk: 'backdrop-dusk.jpg',
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
