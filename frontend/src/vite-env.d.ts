/// <reference types="svelte" />
/// <reference types="vite/client" />

declare module '*.mjs?url' {
  const src: string;
  export default src;
}

declare module '*.mjs?worker&url' {
  const src: string;
  export default src;
}

