import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { login as loginApi } from '../api';
import { Cpu, Lock, ShieldCheck, ArrowRight } from 'lucide-react';

const Login = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleLogin = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const resp = await loginApi(email, password);
      localStorage.setItem('askworx_token', resp.data.token);
      navigate('/');
    } catch (err) {
      setError('Invalid Access Credentials');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#020617] flex items-center justify-center p-6 relative overflow-hidden">
      {/* Background Cyber Effects */}
      <div className="absolute top-0 right-0 w-[500px] h-[500px] bg-primary/10 blur-[120px] rounded-full -mr-64 -mt-64"></div>
      <div className="absolute bottom-0 left-0 w-[500px] h-[500px] bg-secondary/10 blur-[120px] rounded-full -ml-64 -mb-64"></div>
      <div className="absolute inset-0 bg-[url('https://www.transparenttextures.com/patterns/carbon-fibre.png')] opacity-[0.03]"></div>

      <div className="w-full max-w-[440px] relative z-10 animate-in">
        <div className="text-center mb-12">
           <div className="inline-flex items-center justify-center w-24 h-24 mb-8 transition-transform hover:scale-110">
              <img src="/logo.png" alt="ASKworX Logo" className="w-full h-full object-contain shadow-2xl" />
           </div>
           <h1 className="text-4xl font-black text-white tracking-tighter mb-3 uppercase">ASKworX</h1>
           <div className="flex items-center justify-center gap-3">
              <div className="h-px w-8 bg-white/10"></div>
              <span className="text-[10px] font-black uppercase tracking-[0.4em] text-slate-500">Admin Login</span>
              <div className="h-px w-8 bg-white/10"></div>
           </div>
        </div>

        <div className="bg-white/5 backdrop-blur-2xl p-10 rounded-[40px] border border-white/5 shadow-2xl relative group overflow-hidden">
          <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-transparent via-primary to-transparent opacity-50"></div>
          
          <form onSubmit={handleLogin} className="space-y-8 relative z-10">
            <div className="space-y-6">
               <div className="relative group">
                  <div className="absolute left-5 top-1/2 -translate-y-1/2 text-slate-500 group-focus-within:text-primary transition-colors">
                     <Lock className="w-5 h-5" />
                  </div>
                  <input
                    type="email"
                    placeholder="Email"
                    className="w-full bg-white/5 border border-white/10 rounded-[20px] pl-14 pr-6 py-5 text-white font-bold outline-none focus:border-primary/50 focus:ring-4 focus:ring-primary/10 transition-all placeholder:text-slate-600"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                  />
               </div>
               <div className="relative group">
                  <div className="absolute left-5 top-1/2 -translate-y-1/2 text-slate-500 group-focus-within:text-primary transition-colors">
                     <Lock className="w-5 h-5" />
                  </div>
                  <input
                    type="password"
                    placeholder="Password"
                    className="w-full bg-white/5 border border-white/10 rounded-[20px] pl-14 pr-6 py-5 text-white font-bold outline-none focus:border-primary/50 focus:ring-4 focus:ring-primary/10 transition-all placeholder:text-slate-600"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                  />
               </div>
            </div>

            {error && (
               <div className="bg-red-500/10 border border-red-500/20 p-4 rounded-2xl flex items-center gap-3 animate-shake">
                  <div className="w-1.5 h-1.5 rounded-full bg-red-500"></div>
                  <p className="text-[11px] font-black uppercase tracking-widest text-red-400">Invalid Password</p>
               </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full bg-gradient-to-r from-primary via-indigo-600 to-secondary text-white font-black py-5 rounded-[22px] shadow-xl shadow-primary/20 hover:shadow-primary/40 transition-all flex items-center justify-center gap-3 uppercase tracking-widest text-[11px] disabled:opacity-50 relative overflow-hidden group/btn"
            >
              <div className="absolute inset-0 bg-white/10 translate-y-full group-hover/btn:translate-y-0 transition-transform duration-300"></div>
              <span className="relative z-10">{loading ? 'Logging in...' : 'Login Now'}</span>
              {!loading && <ArrowRight className="w-4 h-4 relative z-10 group-hover/btn:translate-x-1 transition-transform" />}
            </button>
          </form>
        </div>

        <div className="mt-10 flex flex-col items-center gap-6">
           <div className="flex items-center gap-2 text-slate-600">
              <ShieldCheck className="w-4 h-4 opacity-30" />
              <span className="text-[9px] font-black uppercase tracking-[0.2em] opacity-40">AES-256 Encrypted Protocol</span>
           </div>
           <p className="text-[10px] text-slate-700 font-bold uppercase tracking-widest text-center">
             Proprietary Intelligence System of <br/>
             <span className="text-slate-500">ASKworX Smart Automation</span>
           </p>
        </div>
      </div>
    </div>
  );
};

export default Login;
