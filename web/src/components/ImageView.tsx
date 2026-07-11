import type { Image as ImageBlock } from '../schema';
import { withToken } from '../token';

// asset:<sha256> resolves to the daemon's content-addressed store at
// /assets/<sha256>, token-bearing so off-loopback sessions authenticate; https:
// and data: URIs pass through unchanged.
function resolveSrc(src: string): string {
  return src.startsWith('asset:') ? withToken(`/assets/${src.slice('asset:'.length)}`) : src;
}

export function ImageView({ block }: { block: ImageBlock }) {
  return (
    <figure className="image-block">
      <img src={resolveSrc(block.src)} alt={block.alt} loading="lazy" />
      {block.caption && <figcaption className="image-caption">{block.caption}</figcaption>}
    </figure>
  );
}
