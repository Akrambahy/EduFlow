/**
 * Demo tracks data.
 * MOCK DATA: Replace with GET /tracks from backend.
 */
export const demoTracks = [
  {
    id: 'trk_fullstack',
    name: 'Full-Stack Development',
    description: 'Master both frontend and backend development with modern technologies.',
    courses: ['crs_react', 'crs_go', 'crs_api', 'crs_db'],
    totalStudents: 24,
    mentorId: 'usr_mentor_01',
    trackerId: 'usr_tracker_01',
    status: 'active',
  },
  {
    id: 'trk_frontend',
    name: 'Frontend Engineering',
    description: 'Build beautiful, accessible, and performant user interfaces.',
    courses: ['crs_react', 'crs_css', 'crs_testing'],
    totalStudents: 18,
    mentorId: 'usr_mentor_01',
    trackerId: 'usr_tracker_01',
    status: 'active',
  },
  {
    id: 'trk_backend',
    name: 'Backend Engineering',
    description: 'Design and build scalable server-side applications and APIs.',
    courses: ['crs_go', 'crs_api', 'crs_db'],
    totalStudents: 16,
    mentorId: 'usr_mentor_01',
    trackerId: 'usr_tracker_01',
    status: 'active',
  },
];

