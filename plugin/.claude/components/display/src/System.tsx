import { useEffect, useRef } from 'react';
import rough from 'roughjs';
import { usePackState } from './host/present';
import type { PackComponentProps } from './host/present';
import { mount } from './system-map.js';
import type { MapHandle, SystemMap, View } from './system-map.js';

export function System({ block }: PackComponentProps) {
  const host = useRef<HTMLDivElement>(null);
  const handle = useRef<MapHandle | null>(null);
  const [view, setView] = usePackState<View>(`${block.id}:view`, {});
  const keep = useRef(setView);
  keep.current = setView;
  const first = useRef(view);
  const map = block as unknown as SystemMap;

  useEffect(() => {
    if (!host.current) return;
    const h = mount(host.current, map, { rough, state: first.current, onState: (v) => keep.current(v) });
    handle.current = h;
    return () => {
      h.destroy();
      handle.current = null;
    };
  }, []);

  useEffect(() => {
    handle.current?.update(map);
  }, [map]);

  return <div className="ccpd-system" ref={host} />;
}
