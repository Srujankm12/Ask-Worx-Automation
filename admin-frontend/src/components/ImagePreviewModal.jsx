import React, { useEffect, useState } from 'react';
import { X, ImageOff, Loader2 } from 'lucide-react';

// Full-size lightbox for a campaign poster image. Follows the same overlay
// pattern as Modal/ConfirmModal: a separate backdrop div behind the content
// so a click on the image itself doesn't bubble into onClose.
const ImagePreviewModal = ({ isOpen, onClose, src, alt }) => {
  const [status, setStatus] = useState('loading'); // 'loading' | 'loaded' | 'error'

  // The component stays mounted (isOpen just toggles the returned output),
  // so a stale status from a previous image would otherwise carry over to
  // the next one.
  useEffect(() => { setStatus('loading'); }, [src]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 animate-in fade-in duration-300">
      <div
        className="absolute inset-0 bg-slate-900/80 backdrop-blur-md"
        onClick={onClose}
      ></div>

      <div className="relative max-w-[90vw] max-h-[90vh] animate-in zoom-in-95 duration-300">
        <button
          onClick={onClose}
          className="absolute -top-4 -right-4 z-10 p-2 bg-white rounded-full shadow-lg text-slate-600 hover:text-slate-900 hover:scale-105 transition-all"
        >
          <X className="w-5 h-5" />
        </button>

        {status === 'error' ? (
          <div className="w-[320px] h-[320px] max-w-[90vw] max-h-[90vh] bg-slate-800 rounded-2xl flex flex-col items-center justify-center text-slate-400 gap-3">
            <ImageOff className="w-10 h-10" />
            <span className="text-xs font-bold uppercase tracking-widest">Image unavailable</span>
          </div>
        ) : (
          <>
            {status === 'loading' && (
              <div className="w-[320px] h-[320px] max-w-[90vw] max-h-[90vh] bg-slate-800 rounded-2xl flex items-center justify-center text-slate-400">
                <Loader2 className="w-8 h-8 animate-spin" />
              </div>
            )}
            <img
              src={src}
              alt={alt || ''}
              onLoad={() => setStatus('loaded')}
              onError={() => setStatus('error')}
              className={`max-w-[90vw] max-h-[90vh] object-contain rounded-2xl shadow-2xl ${status === 'loading' ? 'hidden' : ''}`}
            />
          </>
        )}
      </div>
    </div>
  );
};

export default ImagePreviewModal;
