import src from '../assets/easypos_logo.png'

/** Logo berlatar krem (token --logo-bg) agar teks gelapnya terbaca di tema terang maupun gelap. */
export function Logo({ className = 'h-10' }: { className?: string }) {
  return <img src={src} alt="easyPOS" className={`w-auto rounded-lg bg-logo ${className}`} />
}
