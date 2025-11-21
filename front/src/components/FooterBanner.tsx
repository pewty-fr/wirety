import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faHeart, faCode } from '@fortawesome/free-solid-svg-icons';
import { faGithub } from '@fortawesome/free-brands-svg-icons';

export default function FooterBanner() {
  return (
    <div className="fixed bottom-0 left-0 right-0 z-20 lg:pl-64">
      <div className="relative overflow-hidden bg-gradient-to-r from-primary-600 via-accent-blue to-primary-600 animate-gradient bg-[length:200%_100%]">
        <div className="px-4 py-3">
          <div className="flex flex-col sm:flex-row items-center justify-center gap-2 sm:gap-4 text-white text-sm">
            <div className="flex items-center gap-2">
              <span>Proudly made with</span>
              <FontAwesomeIcon icon={faHeart} className="text-red-300 animate-pulse" />
              <span>by</span>
              <a
                href="https://pewty.fr"
                target="_blank"
                rel="noopener noreferrer"
                className="font-semibold hover:text-primary-100 transition-colors underline decoration-dotted underline-offset-2"
              >
                Pewty
              </a>
            </div>
            <span className="hidden sm:inline text-white/60">•</span>
            <div className="flex items-center gap-2">
              <FontAwesomeIcon icon={faCode} />
              <span>Free & Open Source</span>
              <a
                href="https://github.com/pewty-fr/wirety"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1.5 font-semibold hover:text-primary-100 transition-colors"
              >
                <FontAwesomeIcon icon={faGithub} />
                <span className="underline decoration-dotted underline-offset-2">GitHub</span>
              </a>
            </div>
          </div>
        </div>
        {/* Subtle overlay for depth */}
        <div className="absolute inset-0 bg-gradient-to-b from-transparent to-black/10 pointer-events-none" />
      </div>
    </div>
  );
}
