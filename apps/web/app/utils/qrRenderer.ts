export type QrModuleStyle = 'square' | 'rounded' | 'dots' | 'extra-rounded' | 'diamond' | 'classy' | 'classy-rounded' | 'soft' | 'star'
export type QrCornerStyle = 'square' | 'rounded' | 'circle' | 'dot' | 'leaf'

export function isFinderModule(row: number, column: number, count: number) {
  return (row < 7 && column < 7) || (row < 7 && column >= count - 7) || (row >= count - 7 && column < 7)
}

export function drawQrModule(context: CanvasRenderingContext2D, x: number, y: number, size: number, style: QrModuleStyle, row: number, column: number) {
  const centerX = x + size / 2; const centerY = y + size / 2
  if (style === 'square') { context.fillRect(x, y, size + 0.15, size + 0.15); return }
  if (style === 'dots' || (style === 'classy-rounded' && (row + column) % 3 === 0)) { context.beginPath(); context.arc(centerX, centerY, size * 0.42, 0, Math.PI * 2); context.fill(); return }
  if (style === 'diamond') { context.save(); context.translate(centerX, centerY); context.rotate(Math.PI / 4); context.fillRect(-size * 0.34, -size * 0.34, size * 0.68, size * 0.68); context.restore(); return }
  if (style === 'star') { drawStar(context, centerX, centerY, size * 0.47, size * 0.22); context.fill(); return }
  const inset = style === 'extra-rounded' ? size * 0.05 : size * 0.08
  const moduleSize = size - inset * 2
  const radius = style === 'extra-rounded' || style === 'soft' ? moduleSize / 2 : style === 'classy' && (row + column) % 2 === 0 ? 0 : size * 0.24
  context.beginPath(); context.roundRect(x + inset, y + inset, moduleSize, moduleSize, radius); context.fill()
}

export function drawQrFinders(context: CanvasRenderingContext2D, count: number, margin: number, cell: number, style: QrCornerStyle, foreground: string, background: string) {
  for (const [column, row] of [[0, 0], [count - 7, 0], [0, count - 7]]) {
    const x = (column! + margin) * cell; const y = (row! + margin) * cell
    context.fillStyle = foreground; finderShape(context, x, y, cell * 7, style, 'outer'); context.fill()
    context.fillStyle = background; finderShape(context, x + cell, y + cell, cell * 5, style, 'middle'); context.fill()
    context.fillStyle = foreground; finderShape(context, x + cell * 2, y + cell * 2, cell * 3, style, 'inner'); context.fill()
  }
}

function finderShape(context: CanvasRenderingContext2D, x: number, y: number, size: number, style: QrCornerStyle, layer: 'outer' | 'middle' | 'inner') {
  context.beginPath()
  if (style === 'circle' || (style === 'dot' && layer === 'inner')) { context.arc(x + size / 2, y + size / 2, size / 2, 0, Math.PI * 2); return }
  const radius = style === 'rounded' || style === 'dot' ? size * 0.2 : 0
  if (style === 'leaf') { context.roundRect(x, y, size, size, [size * 0.34, 0, size * 0.34, 0]); return }
  context.roundRect(x, y, size, size, radius)
}

function drawStar(context: CanvasRenderingContext2D, x: number, y: number, outer: number, inner: number) {
  context.beginPath()
  for (let point = 0; point < 10; point++) { const radius = point % 2 ? inner : outer; const angle = -Math.PI / 2 + point * Math.PI / 5; const px = x + Math.cos(angle) * radius; const py = y + Math.sin(angle) * radius; if (point === 0) context.moveTo(px, py); else context.lineTo(px, py) }
  context.closePath()
}

export function qrSvgModule(x: number, y: number, style: QrModuleStyle, row: number, column: number) {
  if (style === 'dots' || (style === 'classy-rounded' && (row + column) % 3 === 0)) return `<circle cx="${x + 0.5}" cy="${y + 0.5}" r=".42"/>`
  if (style === 'diamond') return `<rect x="${x + 0.16}" y="${y + 0.16}" width=".68" height=".68" transform="rotate(45 ${x + 0.5} ${y + 0.5})"/>`
  if (style === 'star') { const points = Array.from({ length: 10 }, (_, point) => { const radius = point % 2 ? 0.22 : 0.47; const angle = -Math.PI / 2 + point * Math.PI / 5; return `${x + 0.5 + Math.cos(angle) * radius},${y + 0.5 + Math.sin(angle) * radius}` }).join(' '); return `<polygon points="${points}"/>` }
  const inset = style === 'square' ? 0 : style === 'extra-rounded' ? 0.05 : 0.08; const size = 1 - inset * 2
  const radius = style === 'extra-rounded' || style === 'soft' ? size / 2 : style === 'classy' && (row + column) % 2 === 0 ? 0 : style === 'square' ? 0 : 0.24
  return `<rect x="${x + inset}" y="${y + inset}" width="${size + (style === 'square' ? 0.01 : 0)}" height="${size + (style === 'square' ? 0.01 : 0)}" rx="${radius}"/>`
}

export function qrSvgFinders(count: number, margin: number, style: QrCornerStyle, foreground: string, background: string) {
  return [[0, 0], [count - 7, 0], [0, count - 7]].map(([column, row]) => {
    const x = column! + margin; const y = row! + margin
    return svgFinderShape(x, y, 7, style, 'outer', foreground) + svgFinderShape(x + 1, y + 1, 5, style, 'middle', background) + svgFinderShape(x + 2, y + 2, 3, style, 'inner', foreground)
  }).join('')
}

function svgFinderShape(x: number, y: number, size: number, style: QrCornerStyle, layer: 'outer' | 'middle' | 'inner', fill: string) {
  if (style === 'circle' || (style === 'dot' && layer === 'inner')) return `<circle cx="${x + size / 2}" cy="${y + size / 2}" r="${size / 2}" fill="${fill}"/>`
  const radius = style === 'rounded' || style === 'dot' ? size * 0.2 : 0
  if (style === 'leaf') return `<path d="M ${x + size * 0.34} ${y} H ${x + size} V ${y + size * 0.66} A ${size * 0.34} ${size * 0.34} 0 0 1 ${x + size * 0.66} ${y + size} H ${x} V ${y + size * 0.34} A ${size * 0.34} ${size * 0.34} 0 0 1 ${x + size * 0.34} ${y} Z" fill="${fill}"/>`
  return `<rect x="${x}" y="${y}" width="${size}" height="${size}" rx="${radius}" fill="${fill}"/>`
}
