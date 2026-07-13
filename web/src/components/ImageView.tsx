import { useState } from 'react';
import type { Image as ImageBlock } from '../schema';
import { withToken } from '../token';
import { Lightbox } from './Lightbox';

// asset:<sha256> resolves to the daemon's content-addressed store at
// /assets/<sha256>, token-bearing so off-loopback sessions authenticate; https:
// and data: URIs pass through unchanged.
function resolveSrc(src: string): string {
  return src.startsWith('asset:') ? withToken(`/assets/${src.slice('asset:'.length)}`) : src;
}

export function ImageView({ block }: { block: ImageBlock }) {
  const [open, setOpen] = useState(false);
  const src = resolveSrc(block.src);
  return (
    <figure className="image-block">
      <button type="button" className="image-trigger" aria-haspopup="dialog" onClick={() => setOpen(true)}>
        <img src={src} alt={block.alt} loading="lazy" />
      </button>
      {block.caption && <figcaption className="image-caption">{block.caption}</figcaption>}
      <Lightbox open={open} onClose={() => setOpen(false)} src={src} alt={block.alt} caption={block.caption} />
    </figure>
  );
}
