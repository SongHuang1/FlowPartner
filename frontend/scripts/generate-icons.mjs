import sharp from 'sharp'
import path from 'path'
import { fileURLToPath } from 'url'
import { writeFileSync, mkdirSync } from 'fs'
import { execSync } from 'child_process'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const svgPath = path.join(rootDir, 'public', 'flowpartner.svg')
const buildDir = path.join(rootDir, 'build')

async function generatePNGs() {
  console.log('Generating PNG icons...')
  const sizes = [16, 32, 48, 64, 128, 256, 512, 1024]
  for (const size of sizes) {
    await sharp(svgPath).resize(size, size).png().toFile(path.join(buildDir, `icon-${size}.png`))
    console.log(`  Created icon-${size}.png`)
  }
  await sharp(svgPath).resize(512, 512).png().toFile(path.join(buildDir, 'icon.png'))
  console.log('  Created icon.png')
}

async function generateICO() {
  console.log('Generating Windows icon...')
  const pngToIco = (await import('png-to-ico')).default
  const sizes = [16, 32, 48, 64, 128, 256]
  const buffers = await Promise.all(
    sizes.map(s => sharp(svgPath).resize(s, s).png().toBuffer())
  )
  const icoBuf = await pngToIco(buffers)
  writeFileSync(path.join(buildDir, 'icon.ico'), icoBuf)
  console.log('  Created icon.ico')
}

async function generateICNS() {
  console.log('Generating macOS icon...')
  const iconsetDir = path.join(buildDir, 'icon.iconset')
  mkdirSync(iconsetDir, { recursive: true })

  const iconsetEntries = [
    { name: 'icon_16x16.png', size: 16 },
    { name: 'icon_16x16@2x.png', size: 32 },
    { name: 'icon_32x32.png', size: 32 },
    { name: 'icon_32x32@2x.png', size: 64 },
    { name: 'icon_128x128.png', size: 128 },
    { name: 'icon_128x128@2x.png', size: 256 },
    { name: 'icon_256x256.png', size: 256 },
    { name: 'icon_256x256@2x.png', size: 512 },
    { name: 'icon_512x512.png', size: 512 },
    { name: 'icon_512x512@2x.png', size: 1024 },
  ]

  for (const { name, size } of iconsetEntries) {
    await sharp(svgPath).resize(size, size).png().toFile(path.join(iconsetDir, name))
    console.log(`  Created ${name}`)
  }

  try {
    execSync(`iconutil -c icns "${iconsetDir}" -o "${path.join(buildDir, 'icon.icns')}"`, { stdio: 'inherit' })
    console.log('  Created icon.icns')
  } catch {
    console.log('  WARNING: iconutil not available. Run manually on macOS:')
    console.log(`    iconutil -c icns "${iconsetDir}" -o "${path.join(buildDir, 'icon.icns')}"`)
  }
}

async function main() {
  mkdirSync(buildDir, { recursive: true })
  await generatePNGs()

  const platform = process.platform
  if (platform === 'win32') {
    await generateICO()
  } else if (platform === 'darwin') {
    await generateICNS()
  } else {
    console.log('Linux detected: PNG icons generated. Run on macOS to create .icns if needed.')
  }

  console.log('\nDone! Icons saved to', buildDir)
}

main().catch(err => {
  console.error('Error:', err)
  process.exit(1)
})
