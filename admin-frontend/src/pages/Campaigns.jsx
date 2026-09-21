import React, { useState, useEffect } from 'react';
import {
  Plus, Calendar, Trash2, BarChart2, CheckCircle, Clock,
  XCircle, Radio, Image, Brain, ChevronDown, ChevronUp, Loader2, ImageOff
} from 'lucide-react';
import {
  getCampaigns, createCampaign, deleteCampaign, getCampaignAnalytics
} from '../api';

// Mirrors the backend's own limits (parseCampaignMultipart in admin.go) so the
// user sees the same rule before the request ever leaves the browser.
const MAX_IMAGE_BYTES = 2 * 1024 * 1024;
const ALLOWED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp'];
const IMAGE_SIZE_ERROR = 'Image size must be less than or equal to 2 MB.';
const IMAGE_TYPE_ERROR = 'Only JPG, JPEG, PNG and WEBP images are allowed.';

const STATUS_STYLES = {
  scheduled: 'bg-blue-50 text-blue-700 border border-blue-200',
  sent: 'bg-emerald-50 text-emerald-700 border border-emerald-200',
  cancelled: 'bg-red-50 text-red-600 border border-red-200',
};

const STATUS_ICONS = {
  scheduled: <Clock className="w-3 h-3" />,
  sent: <CheckCircle className="w-3 h-3" />,
  cancelled: <XCircle className="w-3 h-3" />,
};

const EMPTY_QUIZ = {
  type: 'quiz', question: '', option_a: '', option_b: '', option_c: '',
  correct_answer: 'A', explanation: '', youtube_link: '', scheduled_at: '',
};
const EMPTY_POSTER = {
  type: 'poster', image_url: '', caption: '', title: '', description: '', scheduled_at: '',
};

import Modal from '../components/Modal';
import ImagePreviewModal from '../components/ImagePreviewModal';

export default function Campaigns() {
  const [campaigns, setCampaigns] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [formType, setFormType] = useState('quiz');
  const [form, setForm] = useState(EMPTY_QUIZ);
  const [errors, setErrors] = useState({});
  const [submitting, setSubmitting] = useState(false);
  const [expandedId, setExpandedId] = useState(null);
  const [analytics, setAnalytics] = useState({});
  const [uploadSource, setUploadSource] = useState('url'); // 'url' or 'local'
  const [uploading, setUploading] = useState(false);
  const [modal, setModal] = useState({ open: false, title: '', message: '', type: 'success' });
  const [localPreviewUrl, setLocalPreviewUrl] = useState(null);
  const [selectedImage, setSelectedImage] = useState(null);
  const [brokenImageIds, setBrokenImageIds] = useState(() => new Set());

  // Smarter base URL: use current origin if deployed
  const API_BASE = import.meta.env.VITE_API_URL && import.meta.env.VITE_API_URL.includes('localhost')
    ? import.meta.env.VITE_API_URL
    : window.location.origin;

  // Campaign image_url is stored as the path the backend serves it at
  // (e.g. "/api/campaigns/3/image") whenever PUBLIC_URL isn't configured, so
  // it needs the backend origin prefixed before it's usable in an <img src>.
  // A pasted remote URL, or one already made absolute via PUBLIC_URL, is
  // used as-is.
  const resolveImageUrl = (url) => {
    if (!url) return null;
    return url.startsWith('/') ? `${API_BASE}${url}` : url;
  };

  // Revoke the local preview object URL whenever it's replaced or the form
  // unmounts, so we don't leak blob URLs.
  useEffect(() => {
    return () => {
      if (localPreviewUrl) URL.revokeObjectURL(localPreviewUrl);
    };
  }, [localPreviewUrl]);

  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);

  const load = async () => {
    try {
      setLoading(true);
      const { data } = await getCampaigns({
        limit: 10,
        offset: page * 10
      });
      setCampaigns(data.data || []);
      setTotal(data.total || 0);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [page]);

  const switchType = (t) => {
    setFormType(t);
    setForm(t === 'quiz' ? { ...EMPTY_QUIZ } : { ...EMPTY_POSTER });
    setErrors({});
    if (localPreviewUrl) URL.revokeObjectURL(localPreviewUrl);
    setLocalPreviewUrl(null);
  };

  const validate = () => {
    const e = {};
    if (!form.scheduled_at) e.scheduled_at = 'Schedule date & time required';

    if (formType === 'quiz') {
      if (!form.question.trim()) e.question = 'Question is required';
      if (!form.option_a.trim()) e.option_a = 'Option A required';
      if (!form.option_b.trim()) e.option_b = 'Option B required';
      if (!form.option_c.trim()) e.option_c = 'Option C required';
      if (!form.explanation.trim()) e.explanation = 'Explanation required';
      if (form.explanation.length > 300) e.explanation = 'Max 300 characters';
    }

    if (formType === 'poster') {
      if (!form.title || !form.title.trim()) e.title = 'Title is required';
      if (!form.description || !form.description.trim()) e.description = 'Description is required';

      if (uploadSource === 'url' && !form.image_url.trim()) {
        e.image_url = 'Image URL required';
      }
      if (uploadSource === 'local') {
        if (!form.localFile) {
          e.local_file = 'Image file required';
        } else if (!ALLOWED_IMAGE_TYPES.includes(form.localFile.type)) {
          e.local_file = IMAGE_TYPE_ERROR;
        } else if (form.localFile.size > MAX_IMAGE_BYTES) {
          e.local_file = IMAGE_SIZE_ERROR;
        }
      }
    }

    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = async (ev) => {
    ev.preventDefault();
    if (!validate()) return;
    setSubmitting(true);

    try {
      // Convert datetime-local (YYYY-MM-DDTHH:MM) to full ISO string for Go's time.Time
      const scheduledISO = new Date(form.scheduled_at).toISOString();

      if (formType === 'poster' && uploadSource === 'local') {
        // Send the image straight to POST /campaigns as multipart/form-data —
        // the backend stores it in campaigns.image_data (Postgres BYTEA) and
        // points image_url at the endpoint that serves it back. There is no
        // separate "upload" step; a two-step upload-then-JSON flow was how
        // the poster image ended up saved to local disk under the frontend's
        // own origin instead of the database, which is why it never loaded.
        const fd = new FormData();
        fd.append('type', formType);
        fd.append('title', form.title || '');
        fd.append('description', form.description || '');
        fd.append('caption', form.caption || '');
        fd.append('scheduled_at', scheduledISO);
        fd.append('image', form.localFile);
        await createCampaign(fd);
      } else {
        await createCampaign({ ...form, type: formType, scheduled_at: scheduledISO });
      }

      setModal({
        open: true,
        title: 'Campaign Scheduled! 🚀',
        message: 'Your broadcast has been successfully queued in the system.',
        type: 'success'
      });
      setShowForm(false);
      setForm(EMPTY_QUIZ);
      if (localPreviewUrl) URL.revokeObjectURL(localPreviewUrl);
      setLocalPreviewUrl(null);
      load();
    } catch (e) {
      setModal({
        open: true,
        title: 'Creation Failed',
        message: e?.response?.data?.error || 'Failed to create campaign. Please try again.',
        type: 'error'
      });
    } finally {
      setSubmitting(false);
      setUploading(false);
    }
  };

  const handleDelete = async (id) => {
    if (!window.confirm('Are you sure you want to cancel this campaign?')) return;
    try {
      await deleteCampaign(id);
      load();
    } catch (err) {
      setModal({
        open: true,
        title: 'Cancellation Failed',
        message: 'There was an issue stopping the campaign. Please refresh and try again.',
        type: 'error'
      });
    }
  };

  const toggleAnalytics = async (camp) => {
    if (expandedId === camp.id) { setExpandedId(null); return; }
    setExpandedId(camp.id);
    if (!analytics[camp.id]) {
      try {
        const { data } = await getCampaignAnalytics(camp.id);
        setAnalytics(prev => ({ ...prev, [camp.id]: data }));
      } catch { }
    }
  };

  const field = (key, label, opts = {}) => (
    <div key={key}>
      <label className="block text-xs font-semibold text-slate-500 mb-1">{label}</label>
      {opts.textarea ? (
        <textarea
          rows={3}
          maxLength={300}
          className={`w-full border rounded-xl px-3 py-2 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-400 resize-none ${errors[key] ? 'border-red-400' : 'border-slate-200'}`}
          value={form[key] || ''}
          onChange={e => setForm(f => ({ ...f, [key]: e.target.value }))}
          placeholder={opts.placeholder || ''}
        />
      ) : (
        <input
          type={opts.type || 'text'}
          className={`w-full border rounded-xl px-3 py-2 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-400 ${errors[key] ? 'border-red-400' : 'border-slate-200'}`}
          value={form[key] || ''}
          onChange={e => setForm(f => ({ ...f, [key]: e.target.value }))}
          placeholder={opts.placeholder || ''}
        />
      )}
      {errors[key] && <p className="text-xs text-red-500 mt-1">{errors[key]}</p>}
    </div>
  );

  return (
    <div className="p-6 lg:p-10 max-w-5xl mx-auto">
      <Modal
        isOpen={modal.open}
        onClose={() => setModal({ ...modal, open: false })}
        title={modal.title}
        message={modal.message}
        type={modal.type}
      />
      <ImagePreviewModal
        isOpen={!!selectedImage}
        onClose={() => setSelectedImage(null)}
        src={selectedImage ? resolveImageUrl(selectedImage.image_url) : null}
        alt={selectedImage?.title || 'Poster'}
      />
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-black text-slate-900">Campaigns</h1>
          <p className="text-sm text-slate-500 mt-1">Schedule weekly quizzes & poster broadcasts</p>
        </div>
        <button
          onClick={() => { setShowForm(!showForm); setForm(EMPTY_QUIZ); setErrors({}); }}
          className="flex items-center gap-2 bg-indigo-600 hover:bg-indigo-700 text-white px-4 py-2.5 rounded-xl text-sm font-semibold shadow transition-all active:scale-95"
        >
          <Plus className="w-4 h-4" />
          New Campaign
        </button>
      </div>

      {/* Create Form */}
      {showForm && (
        <div className="bg-white border border-slate-200 rounded-2xl shadow-lg p-6 mb-8 animate-in">
          <h2 className="text-lg font-black text-slate-800 mb-5">Create Campaign</h2>

          {/* Type Toggle */}
          <div className="flex gap-3 mb-6">
            {[['quiz', Brain, 'Quiz'], ['poster', Image, 'Poster']].map(([t, Icon, label]) => (
              <button
                key={t}
                type="button"
                onClick={() => switchType(t)}
                className={`flex items-center gap-2 px-5 py-2.5 rounded-xl border text-sm font-semibold transition-all ${formType === t
                    ? 'bg-indigo-600 text-white border-indigo-600 shadow'
                    : 'bg-white text-slate-600 border-slate-200 hover:border-indigo-300'
                  }`}
              >
                <Icon className="w-4 h-4" /> {label}
              </button>
            ))}
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            {formType === 'quiz' ? (
              <>
                {field('question', 'Quiz Question', { placeholder: 'What is a PLC?' })}
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  {field('option_a', 'Option A', { placeholder: 'First option' })}
                  {field('option_b', 'Option B', { placeholder: 'Second option' })}
                  {field('option_c', 'Option C', { placeholder: 'Third option' })}
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-500 mb-1">Correct Answer</label>
                  <select
                    className="border border-slate-200 rounded-xl px-3 py-2 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-400"
                    value={form.correct_answer}
                    onChange={e => setForm(f => ({ ...f, correct_answer: e.target.value }))}
                  >
                    <option value="A">A</option>
                    <option value="B">B</option>
                    <option value="C">C</option>
                  </select>
                </div>
                {field('explanation', 'Explanation (max 300 chars)', { textarea: true, placeholder: 'Explain the answer in 2–3 lines...' })}
                <p className="text-xs text-slate-400">{(form.explanation || '').length}/300 characters</p>
                {field('youtube_link', 'YouTube Link (optional)', { placeholder: 'https://youtube.com/...' })}
              </>
            ) : (
              <>
                <div className="flex gap-4 p-1 bg-slate-100 rounded-xl w-fit mb-4">
                  {[['url', 'Remote URL'], ['local', 'Local Upload']].map(([s, l]) => (
                    <button
                      key={s}
                      type="button"
                      onClick={() => setUploadSource(s)}
                      className={`px-4 py-1.5 rounded-lg text-xs font-bold transition-all ${uploadSource === s ? 'bg-white text-indigo-600 shadow-sm' : 'text-slate-500'}`}
                    >
                      {l}
                    </button>
                  ))}
                </div>

                {field('title', 'Title', { placeholder: 'New service launch' })}
                {field('description', 'Description', { textarea: true, placeholder: 'Describe what this poster is announcing...' })}

                {uploadSource === 'url' ? (
                  field('image_url', 'Image URL', { placeholder: 'https://images.unsplash.com/...' })
                ) : (
                  <div>
                    <label className="block text-xs font-semibold text-slate-500 mb-1">Select Image File</label>
                    <input
                      type="file"
                      accept="image/jpeg,image/png,image/webp"
                      onChange={(e) => {
                        const file = e.target.files[0];
                        if (localPreviewUrl) URL.revokeObjectURL(localPreviewUrl);
                        setLocalPreviewUrl(null);
                        setErrors(prev => ({ ...prev, local_file: undefined }));

                        if (!file) {
                          setForm(f => ({ ...f, localFile: null }));
                          return;
                        }
                        if (!ALLOWED_IMAGE_TYPES.includes(file.type)) {
                          setErrors(prev => ({ ...prev, local_file: IMAGE_TYPE_ERROR }));
                          setForm(f => ({ ...f, localFile: null }));
                          e.target.value = '';
                          return;
                        }
                        if (file.size > MAX_IMAGE_BYTES) {
                          setErrors(prev => ({ ...prev, local_file: IMAGE_SIZE_ERROR }));
                          setForm(f => ({ ...f, localFile: null }));
                          e.target.value = '';
                          return;
                        }
                        setForm(f => ({ ...f, localFile: file }));
                        setLocalPreviewUrl(URL.createObjectURL(file));
                      }}
                      className={`w-full border border-dashed rounded-xl p-4 bg-slate-50 text-sm file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-xs file:font-bold file:bg-indigo-50 file:text-indigo-700 hover:file:bg-indigo-100 ${errors.local_file ? 'border-red-400' : 'border-slate-300'}`}
                    />
                    {errors.local_file && <p className="mt-2 text-xs text-red-500 font-semibold">{errors.local_file}</p>}
                    {form.localFile && !errors.local_file && (
                      <p className="mt-2 text-xs text-emerald-600 font-semibold flex items-center gap-1"><CheckCircle className="w-3 h-3" /> {form.localFile.name} selected</p>
                    )}
                    {localPreviewUrl && (
                      <img
                        src={localPreviewUrl}
                        alt="Selected poster preview"
                        className="mt-3 max-h-48 rounded-xl border border-slate-200 object-contain"
                      />
                    )}
                  </div>
                )}
                {field('caption', 'Caption (optional)', { textarea: true, placeholder: 'Add a message to accompany the image...' })}
              </>
            )}

            <div>
              <label className="block text-xs font-semibold text-slate-500 mb-1">Schedule Date & Time (IST)</label>
              <input
                type="datetime-local"
                className={`border rounded-xl px-3 py-2 text-sm bg-white focus:outline-none focus:ring-2 focus:ring-indigo-400 ${errors.scheduled_at ? 'border-red-400' : 'border-slate-200'}`}
                value={form.scheduled_at}
                onChange={e => setForm(f => ({ ...f, scheduled_at: e.target.value }))}
              />
              {errors.scheduled_at && <p className="text-xs text-red-500 mt-1">{errors.scheduled_at}</p>}
            </div>

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={submitting}
                className="flex items-center gap-2 bg-indigo-600 hover:bg-indigo-700 text-white px-6 py-2.5 rounded-xl text-sm font-semibold shadow transition-all active:scale-95 disabled:opacity-60"
              >
                {submitting ? <Loader2 className="w-4 h-4 animate-spin" /> : <Calendar className="w-4 h-4" />}
                Schedule Campaign
              </button>
              <button
                type="button"
                onClick={() => setShowForm(false)}
                className="px-5 py-2.5 rounded-xl border border-slate-200 text-sm font-semibold text-slate-600 hover:bg-slate-50 transition-all"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Campaign List */}
      {loading ? (
        <div className="flex items-center justify-center py-24 text-slate-400">
          <Loader2 className="w-7 h-7 animate-spin mr-3" /> Loading campaigns...
        </div>
      ) : campaigns.length === 0 ? (
        <div className="text-center py-24">
          <Radio className="w-12 h-12 text-slate-300 mx-auto mb-4" />
          <p className="text-slate-500 font-semibold">No campaigns yet</p>
          <p className="text-sm text-slate-400 mt-1">Click "New Campaign" to schedule your first quiz or poster broadcast.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {campaigns.map(camp => {
            const a = analytics[camp.id];
            const isExpanded = expandedId === camp.id;
            const correctPct = a && a.total_answers > 0
              ? Math.round((a.correct / a.total_answers) * 100) : 0;

            return (
              <div key={camp.id} className="bg-white border border-slate-200 rounded-2xl shadow-sm overflow-hidden">
                <div className="flex items-start gap-4 p-5">
                  {/* Type Icon / Poster Thumbnail */}
                  {camp.type === 'poster' && camp.image_url && !brokenImageIds.has(camp.id) ? (
                    <button
                      type="button"
                      onClick={() => setSelectedImage(camp)}
                      className="w-10 h-10 rounded-xl overflow-hidden shrink-0 border border-slate-200 hover:ring-2 hover:ring-indigo-400 transition-all"
                      title="View full image"
                    >
                      <img
                        src={resolveImageUrl(camp.image_url)}
                        alt={camp.title || 'Poster'}
                        className="w-full h-full object-cover"
                        onError={() => {
                          console.error(`[Campaigns] failed to load image for campaign ${camp.id}: ${resolveImageUrl(camp.image_url)}`);
                          setBrokenImageIds(prev => new Set(prev).add(camp.id));
                        }}
                      />
                    </button>
                  ) : (
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${camp.type === 'quiz' ? 'bg-indigo-100' : 'bg-amber-100'}`}>
                      {camp.type === 'quiz'
                        ? <Brain className="w-5 h-5 text-indigo-600" />
                        : camp.type === 'poster' && camp.image_url
                          ? <ImageOff className="w-5 h-5 text-amber-600" />
                          : <Image className="w-5 h-5 text-amber-600" />}
                    </div>
                  )}

                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap mb-1">
                      <span className={`inline-flex items-center gap-1 text-xs font-semibold px-2 py-0.5 rounded-full ${STATUS_STYLES[camp.status]}`}>
                        {STATUS_ICONS[camp.status]}
                        {camp.status.charAt(0).toUpperCase() + camp.status.slice(1)}
                      </span>
                      <span className="text-xs text-slate-400 font-medium uppercase tracking-wide">{camp.type}</span>
                    </div>
                    <p className="font-semibold text-slate-800 text-sm truncate">
                      {camp.type === 'quiz' ? camp.question : (camp.title || camp.caption || camp.image_url)}
                    </p>
                    <p className="text-xs text-slate-400 mt-1">
                      <Calendar className="w-3 h-3 inline mr-1" />
                      {new Date(camp.scheduled_at).toLocaleString('en-IN', { dateStyle: 'medium', timeStyle: 'short' })}
                      {camp.status === 'sent' && ` · Sent to ${camp.total_sent} contacts`}
                    </p>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    {camp.status === 'sent' && camp.type === 'quiz' && (
                      <button
                        onClick={() => toggleAnalytics(camp)}
                        className="flex items-center gap-1 text-xs font-semibold text-indigo-600 hover:text-indigo-800 px-3 py-1.5 rounded-lg hover:bg-indigo-50 transition-all"
                      >
                        <BarChart2 className="w-3.5 h-3.5" />
                        Stats
                        {isExpanded ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                      </button>
                    )}
                    {camp.status === 'scheduled' && (
                      <button
                        onClick={() => handleDelete(camp.id)}
                        className="p-1.5 text-red-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-all"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>

                {/* Analytics Panel */}
                {isExpanded && (
                  <div className="border-t border-slate-100 bg-slate-50 px-5 py-4">
                    {!a ? (
                      <div className="flex items-center gap-2 text-slate-400 text-sm">
                        <Loader2 className="w-4 h-4 animate-spin" /> Loading analytics...
                      </div>
                    ) : (
                      <div>
                        <h3 className="text-xs font-black uppercase tracking-widest text-slate-500 mb-3">Quiz Performance</h3>
                        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
                          {[
                            ['Total Sent', a.total_sent, 'bg-slate-100 text-slate-700'],
                            ['Responses', a.total_answers, 'bg-blue-100 text-blue-700'],
                            ['✅ Correct', a.correct, 'bg-emerald-100 text-emerald-700'],
                            ['❌ Incorrect', a.incorrect, 'bg-red-100 text-red-700'],
                          ].map(([label, val, cls]) => (
                            <div key={label} className={`rounded-xl p-3 text-center ${cls}`}>
                              <p className="text-xl font-black">{val ?? 0}</p>
                              <p className="text-xs font-semibold mt-0.5">{label}</p>
                            </div>
                          ))}
                        </div>

                        {a.total_answers > 0 && (
                          <div className="space-y-6">
                            <div>
                              <p className="text-xs font-semibold text-slate-500 mb-2">Answer Distribution</p>
                              <div className="space-y-2">
                                {[['A', a.answer_a], ['B', a.answer_b], ['C', a.answer_c]].map(([opt, cnt]) => {
                                  const pct = Math.round((cnt / a.total_answers) * 100);
                                  return (
                                    <div key={opt} className="flex items-center gap-3">
                                      <span className="w-6 text-xs font-bold text-slate-600">{opt}</span>
                                      <div className="flex-1 bg-slate-200 rounded-full h-2">
                                        <div className="bg-indigo-500 h-2 rounded-full transition-all" style={{ width: `${pct}%` }} />
                                      </div>
                                      <span className="text-xs text-slate-500 w-10 text-right">{cnt} ({pct}%)</span>
                                    </div>
                                  );
                                })}
                              </div>
                            </div>

                            <div>
                              <p className="text-xs font-semibold text-slate-500 mb-2">Respondent List</p>
                              <div className="bg-white border border-slate-200 rounded-xl overflow-hidden">
                                <table className="w-full text-left text-xs">
                                  <thead>
                                    <tr className="bg-slate-50 border-b border-slate-100">
                                      <th className="px-3 py-2 font-bold text-slate-600">Name / Phone</th>
                                      <th className="px-3 py-2 font-bold text-slate-600 text-center">Choice</th>
                                      <th className="px-3 py-2 font-bold text-slate-600 text-right">Result</th>
                                    </tr>
                                  </thead>
                                  <tbody className="divide-y divide-slate-50">
                                    {(a.responses || []).map((resp, i) => (
                                      <tr key={i} className="hover:bg-slate-50 transition-all">
                                        <td className="px-3 py-2">
                                          <div className="font-semibold text-slate-800">{resp.name || 'Unknown'}</div>
                                          <div className="text-slate-400 font-mono">+{resp.phone}</div>
                                        </td>
                                        <td className="px-3 py-2 text-center">
                                          <span className="inline-block w-6 bg-slate-100 rounded font-black text-slate-600 py-0.5">{resp.answer}</span>
                                        </td>
                                        <td className="px-3 py-2 text-right">
                                          {resp.is_correct ? (
                                            <span className="text-emerald-600 font-bold">✅ CORRECT</span>
                                          ) : (
                                            <span className="text-red-500 font-bold">❌ INCORRECT</span>
                                          )}
                                        </td>
                                      </tr>
                                    ))}
                                  </tbody>
                                </table>
                              </div>
                            </div>

                            <p className="text-xs text-slate-400">
                              Overall accuracy: <strong className="text-emerald-600">{correctPct}%</strong>
                            </p>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Pagination Controls */}
      {total > 10 && (
        <div className="mt-10 px-6 py-4 bg-white border border-slate-200 rounded-2xl flex items-center justify-between shadow-sm">
          <span className="text-[10px] font-black uppercase tracking-widest text-slate-400">
            Showing Page <span className="text-slate-900">{page + 1}</span> of {Math.ceil(total / 10)}
          </span>
          <div className="flex gap-6 items-center">
            <button
              disabled={page === 0}
              onClick={() => setPage(page - 1)}
              className="text-[10px] font-black uppercase tracking-widest text-slate-400 hover:text-indigo-600 disabled:opacity-30 transition-all"
            >
              Previous
            </button>
            <span className="w-px h-4 bg-slate-200" />
            <button
              disabled={(page + 1) * 10 >= total}
              onClick={() => setPage(page + 1)}
              className="text-[10px] font-black uppercase tracking-widest text-slate-400 hover:text-indigo-600 disabled:opacity-30 transition-all"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
