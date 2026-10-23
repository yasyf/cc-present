// jsdom has no layout, and its Range lacks getClientRects; Clamped measures
// line boxes through it, so an unmeasured range reports no boxes. Node-env test
// files have no Range at all.
if (typeof Range !== 'undefined') {
  Range.prototype.getClientRects ??= () => [] as unknown as DOMRectList;
}
