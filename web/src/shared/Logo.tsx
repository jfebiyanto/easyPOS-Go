import light from '../assets/easypos_logo_light.png'
import dark from '../assets/easypos_logo_dark.png'
import circle from '../assets/easypos_logo_circle.png'

/** Logo transparan; varian teks terang dipilih lewat CSS (.logo-light / .logo-dark di theme.css). */
export function Logo({ className = 'h-10 w-auto' }: { className?: string }) {
  return (
    <>
      <img src={light} alt="easyPOS" className={`logo-light ${className}`} />
      <img src={dark} alt="easyPOS" className={`logo-dark ${className}`} />
    </>
  )
}

export function LogoMark({ className = 'h-12 w-auto' }: { className?: string }) {
  return <img src={circle} alt="" aria-hidden="true" className={className} />
}
