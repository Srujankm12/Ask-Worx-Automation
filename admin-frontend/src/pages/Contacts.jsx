import React, { useState, useEffect } from 'react';
import { getContacts } from '../api';
import { format } from 'date-fns';
import { formatSlug } from '../utils';
import { UserPlus, Send, Search, X, MessageSquare, Trash2, Bell, BellOff } from 'lucide-react';
import { saveContact, sendMessage, deleteContact, toggleOptOut } from '../api';

const Contacts = () => {
  const [contacts, setContacts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  
  // Add Contact Modal State
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [newContact, setNewContact] = useState({ name: '', phone: '' });
  
  // Send Message Modal State
  const [isMsgModalOpen, setIsMsgModalOpen] = useState(false);
  const [targetContact, setTargetContact] = useState(null);
  const [message, setMessage] = useState('');

  useEffect(() => {
    fetchContacts();
  }, []);

  const fetchContacts = async () => {
    try {
      const resp = await getContacts();
      setContacts(resp.data || []);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const handleAddContact = async (e) => {
    e.preventDefault();
    if (!newContact.phone.trim()) return;
    try {
      await saveContact(newContact);
      setIsAddModalOpen(false);
      setNewContact({ name: '', phone: '' });
      fetchContacts();
    } catch (err) {
      alert('Failed to add contact');
    }
  };

  const handleSendMessage = async (e) => {
    e.preventDefault();
    if (!targetContact || !message.trim()) return;
    try {
      await sendMessage(targetContact.phone, message);
      setIsMsgModalOpen(false);
      setMessage('');
      alert('Message sent successfully');
    } catch (err) {
      alert('Failed to send message');
    }
  };

  const handleDeleteContact = async (id) => {
    if (!window.confirm('Are you sure you want to delete this contact? They will no longer receive marketing broadcasts.')) return;
    try {
      await deleteContact(id);
      fetchContacts();
    } catch (err) {
      alert('Failed to delete contact');
    }
  };

  const handleToggleOptOut = async (id, currentStatus) => {
    try {
      await toggleOptOut(id, !currentStatus);
      fetchContacts();
    } catch (err) {
      alert('Failed to update subscription status');
    }
  };

  const filteredContacts = contacts.filter(c => 
    (c.name || '').toLowerCase().includes(searchTerm.toLowerCase()) || 
    c.phone.includes(searchTerm)
  );

  return (
    <div className="p-10 lg:p-14 max-w-[1800px] mx-auto animate-in h-[calc(100vh-80px)] flex flex-col overflow-hidden">
      <div className="flex justify-between items-end mb-12 shrink-0">
        <div>
          <div className="flex items-center gap-3 mb-3">
            <div className="px-3 py-1 bg-indigo-500/10 rounded-full border border-indigo-500/20">
              <span className="text-[9px] font-black uppercase tracking-[0.3em] text-indigo-600">Contacts</span>
            </div>
          </div>
          <h1 className="text-4xl lg:text-5xl font-black text-slate-900 tracking-tighter leading-none">
            Global <span className="bg-gradient-to-r from-indigo-500 to-purple-600 bg-clip-text text-transparent">Contacts</span>
          </h1>
        </div>
        <div className="text-right flex flex-col items-end gap-4">
          <div className="flex items-center gap-3">
             <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3 h-3 text-slate-400" />
                <input 
                  type="text" 
                  placeholder="Search contacts..." 
                  className="bg-white border border-slate-100 pl-9 pr-4 py-2.5 rounded-xl text-[10px] font-bold outline-none focus:ring-4 focus:ring-indigo-500/5 transition-all w-48"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                />
             </div>
             <button 
               onClick={() => setIsAddModalOpen(true)}
               className="flex items-center gap-2 bg-slate-900 text-white px-5 py-2.5 rounded-xl text-[10px] font-black uppercase tracking-widest hover:bg-indigo-600 transition-all active:scale-95 shadow-lg shadow-slate-900/10"
             >
               <UserPlus className="w-3.5 h-3.5" />
               Add Contact
             </button>
          </div>
          <div>
            <span className="text-[10px] font-black uppercase tracking-widest text-slate-400 block mb-1">Authenticated</span>
            <span className="text-3xl font-black text-indigo-500 tracking-tighter tabular-nums">{filteredContacts.length}</span>
          </div>
        </div>
      </div>

      <div className="premium-card flex-1 overflow-hidden bg-white shadow-sm border-none flex flex-col min-h-0">
        <div className="flex-1 overflow-auto">
          <table className="w-full text-left border-collapse min-w-[800px]">
            <thead className="sticky top-0 z-10">
              <tr className="text-slate-500 text-[9px] font-black uppercase tracking-[0.2em] bg-slate-50/80 backdrop-blur-md">
                <th className="px-10 py-6">User Identity</th>
                <th className="px-10 py-6">Status</th>
                <th className="px-10 py-6">Engagement</th>
                <th className="px-10 py-6 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100/50">
              {filteredContacts.map((contact, idx) => (
                <tr key={contact.id || idx} className="group hover:bg-slate-50/50 transition-all">
                  <td className="px-10 py-8">
                    <div className="flex items-center gap-6">
                      <div className="w-12 h-12 rounded-2xl bg-gradient-to-br from-indigo-500 to-purple-500 flex items-center justify-center text-white font-black text-xs shadow-lg shadow-indigo-500/20 group-hover:scale-110 transition-all shrink-0">
                        {(contact.name || 'A')[0].toUpperCase()}
                      </div>
                      <div className="flex flex-col min-w-0">
                        <span className="font-black text-slate-800 text-sm tracking-tight capitalize truncate">{contact.name ? formatSlug(contact.name) : 'Anonymous User'}</span>
                        <span className="text-[10px] text-slate-500 font-bold uppercase tracking-widest mt-1">+{contact.phone}</span>
                      </div>
                    </div>
                  </td>
                  <td className="px-10 py-8">
                    {contact.opt_out ? (
                      <span className="text-[9px] font-black text-red-400 uppercase tracking-widest bg-red-50 px-2 py-1 rounded-md">Opted Out</span>
                    ) : (
                      <span className="text-[9px] font-black text-green-400 uppercase tracking-widest bg-green-50 px-2 py-1 rounded-md">Subscribed</span>
                    )}
                  </td>
                  <td className="px-10 py-8">
                    <div className="flex gap-1.5">
                      {[1, 2, 3, 4, 5].map((s) => (
                        <div key={s} className={`w-1.5 h-1.5 rounded-full ${s <= 4 ? 'bg-indigo-500/40' : 'bg-slate-100'}`}></div>
                      ))}
                    </div>
                  </td>
                  <td className="px-10 py-8 text-right flex justify-end gap-2">
                    <button 
                      onClick={() => handleToggleOptOut(contact.id, contact.opt_out)}
                      className={`p-3 rounded-xl transition-all active:scale-95 group/btn ${contact.opt_out ? 'bg-green-50 text-green-600 hover:bg-green-600 hover:text-white' : 'bg-orange-50 text-orange-600 hover:bg-orange-600 hover:text-white'}`}
                      title={contact.opt_out ? "Re-subscribe" : "Disable Automated Messages"}
                    >
                      {contact.opt_out ? <Bell className="w-4 h-4 group-hover/btn:scale-110 transition-transform" /> : <BellOff className="w-4 h-4 group-hover/btn:scale-110 transition-transform" />}
                    </button>
                    <button 
                      onClick={() => handleDeleteContact(contact.id)}
                      className="p-3 bg-red-50 text-red-600 rounded-xl hover:bg-red-600 hover:text-white transition-all active:scale-95 group/btn"
                      title="Delete Contact"
                    >
                      <Trash2 className="w-4 h-4 group-hover/btn:scale-110 transition-transform" />
                    </button>
                    <button 
                      onClick={() => {
                        setTargetContact(contact);
                        setIsMsgModalOpen(true);
                      }}
                      className="p-3 bg-indigo-50 text-indigo-600 rounded-xl hover:bg-indigo-600 hover:text-white transition-all active:scale-95 group/btn"
                    >
                      <Send className="w-4 h-4 group-hover/btn:translate-x-0.5 group-hover/btn:-translate-y-0.5 transition-transform" />
                    </button>
                  </td>
                </tr>
              ))}
              {filteredContacts.length === 0 && !loading && (
                <tr>
                  <td colSpan="4" className="px-10 py-20 text-center">
                    <p className="text-[10px] font-black uppercase tracking-[0.3em] text-slate-300">No authenticated contacts found</p>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Contact Modal */}
      {isAddModalOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 bg-slate-900/60 backdrop-blur-sm animate-in fade-in duration-300">
          <div className="bg-white rounded-[40px] w-full max-w-md p-10 shadow-2xl animate-in zoom-in-95 slide-in-from-bottom-10 duration-500 relative">
            <button onClick={() => setIsAddModalOpen(false)} className="absolute top-8 right-8 p-2 hover:bg-slate-50 rounded-xl transition-all text-slate-300 hover:text-slate-900">
              <X className="w-5 h-5" />
            </button>
            <div className="mb-8">
              <div className="w-14 h-14 bg-indigo-50 rounded-2xl flex items-center justify-center mb-6">
                <UserPlus className="w-7 h-7 text-indigo-600" />
              </div>
              <h2 className="text-2xl font-black text-slate-900 tracking-tight">Add New Contact</h2>
              <p className="text-sm font-bold text-slate-400 mt-2">Manual entry for broadcasting</p>
            </div>
            <form onSubmit={handleAddContact} className="space-y-6">
              <div>
                <label className="text-[10px] font-black uppercase tracking-widest text-slate-400 block mb-2 px-1">Full Name</label>
                <input 
                  type="text" 
                  placeholder="e.g. John Doe"
                  className="w-full bg-slate-50 border-none px-6 py-4 rounded-2xl text-xs font-bold outline-none focus:ring-4 focus:ring-indigo-500/10 transition-all"
                  value={newContact.name}
                  onChange={(e) => setNewContact({...newContact, name: e.target.value})}
                />
              </div>
              <div>
                <label className="text-[10px] font-black uppercase tracking-widest text-slate-400 block mb-2 px-1">Phone Number (with country code)</label>
                <input 
                  type="text" 
                  placeholder="e.g. 919876543210"
                  className="w-full bg-slate-50 border-none px-6 py-4 rounded-2xl text-xs font-bold outline-none focus:ring-4 focus:ring-indigo-500/10 transition-all"
                  value={newContact.phone}
                  onChange={(e) => setNewContact({...newContact, phone: e.target.value})}
                  required
                />
              </div>
              <button type="submit" className="w-full py-5 bg-indigo-600 text-white rounded-[24px] font-black text-xs uppercase tracking-[0.3em] shadow-xl shadow-indigo-600/20 hover:scale-[1.02] active:scale-95 transition-all mt-4">
                Save Contact
              </button>
            </form>
          </div>
        </div>
      )}

      {/* Quick Message Modal */}
      {isMsgModalOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 bg-slate-900/60 backdrop-blur-sm animate-in fade-in duration-300">
          <div className="bg-white rounded-[40px] w-full max-w-md p-10 shadow-2xl animate-in zoom-in-95 slide-in-from-bottom-10 duration-500 relative">
            <button onClick={() => setIsMsgModalOpen(false)} className="absolute top-8 right-8 p-2 hover:bg-slate-50 rounded-xl transition-all text-slate-300 hover:text-slate-900">
              <X className="w-5 h-5" />
            </button>
            <div className="mb-8">
              <div className="w-14 h-14 bg-purple-50 rounded-2xl flex items-center justify-center mb-6">
                <MessageSquare className="w-7 h-7 text-purple-600" />
              </div>
              <h2 className="text-2xl font-black text-slate-900 tracking-tight">Quick Message</h2>
              <p className="text-sm font-bold text-slate-400 mt-2">Send to {targetContact?.name || targetContact?.phone}</p>
            </div>
            <form onSubmit={handleSendMessage} className="space-y-6">
              <div>
                <label className="text-[10px] font-black uppercase tracking-widest text-slate-400 block mb-2 px-1">Message Content</label>
                <textarea 
                  rows="4"
                  placeholder="Type your message here..."
                  className="w-full bg-slate-50 border-none px-6 py-4 rounded-2xl text-xs font-bold outline-none focus:ring-4 focus:ring-purple-500/10 transition-all resize-none"
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  required
                ></textarea>
              </div>
              <button type="submit" className="w-full py-5 bg-purple-600 text-white rounded-[24px] font-black text-xs uppercase tracking-[0.3em] shadow-xl shadow-purple-600/20 hover:scale-[1.02] active:scale-95 transition-all mt-4">
                Send Now
              </button>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default Contacts;
