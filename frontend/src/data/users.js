/**
 * Demo users – one per role so every flow is testable.
 *
 * MOCK DATA: Replace with real user objects from GET /users/me
 * once the backend is connected.
 */
export const demoUsers = [
  {
    id: 'usr_admin_01',
    name: 'Sara Admin',
    email: 'admin@eduflow.dev',
    role: 'admin',
    avatar: null,
    university: 'EduFlow Institute',
  },
  {
    id: 'usr_instructor_01',
    name: 'Ahmed Khalil',
    email: 'instructor@eduflow.dev',
    role: 'instructor',
    avatar: null,
    university: 'EduFlow Institute',
    courses: ['crs_react', 'crs_go', 'crs_db'],
  },
  {
    id: 'usr_mentor_01',
    name: 'Layla Hassan',
    email: 'mentor@eduflow.dev',
    role: 'mentor',
    avatar: null,
    university: 'EduFlow Institute',
    tracks: ['trk_fullstack'],
  },
  {
    id: 'usr_tracker_01',
    name: 'Omar Nabil',
    email: 'tracker@eduflow.dev',
    role: 'tracker',
    avatar: null,
    university: 'EduFlow Institute',
    tracks: ['trk_fullstack'],
  },
  {
    id: 'usr_student_01',
    name: 'Marwan Ashour',
    email: 'student@eduflow.dev',
    role: 'student',
    avatar: null,
    university: 'EduFlow Institute',
    track: 'Go Backend',
    enrolledCourses: ['crs_react', 'crs_go', 'crs_db', 'crs_api'],
  },
];

