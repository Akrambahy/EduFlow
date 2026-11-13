/**
 * Demo courses, modules, and lessons data.
 * MOCK DATA: Replace with GET /courses, GET /courses/:id/modules, etc.
 */

export const demoCourses = [
  {
    id: 'crs_react',
    title: 'React Fundamentals',
    description: 'Learn to build dynamic user interfaces with React. Covers components, state, hooks, and routing.',
    category: 'Frontend',
    instructorId: 'usr_instructor_01',
    instructorName: 'Ahmed Khalil',
    duration: '6 weeks',
    totalLessons: 12,
    totalModules: 4,
    enrolledCount: 24,
    status: 'published',
    color: 'teal',
  },
  {
    id: 'crs_go',
    title: 'Go Backend Basics',
    description: 'Build robust backend services with Go. Learn the standard library, concurrency, and web servers.',
    category: 'Backend',
    instructorId: 'usr_instructor_01',
    instructorName: 'Ahmed Khalil',
    duration: '8 weeks',
    totalLessons: 15,
    totalModules: 5,
    enrolledCount: 20,
    status: 'published',
    color: 'emerald',
  },
  {
    id: 'crs_api',
    title: 'RESTful API Design',
    description: 'Design and implement clean, scalable REST APIs following industry best practices.',
    category: 'Backend',
    instructorId: 'usr_instructor_01',
    instructorName: 'Ahmed Khalil',
    duration: '4 weeks',
    totalLessons: 8,
    totalModules: 3,
    enrolledCount: 22,
    status: 'published',
    color: 'sky',
  },
  {
    id: 'crs_db',
    title: 'Database Foundations',
    description: 'Understand relational databases, SQL, schema design, and query optimization.',
    category: 'Data',
    instructorId: 'usr_instructor_01',
    instructorName: 'Ahmed Khalil',
    duration: '5 weeks',
    totalLessons: 10,
    totalModules: 4,
    enrolledCount: 18,
    status: 'published',
    color: 'amber',
  },
  {
    id: 'crs_css',
    title: 'Modern CSS & Layout',
    description: 'Master CSS Grid, Flexbox, animations, and responsive design patterns.',
    category: 'Frontend',
    instructorId: 'usr_instructor_01',
    instructorName: 'Ahmed Khalil',
    duration: '3 weeks',
    totalLessons: 6,
    totalModules: 2,
    enrolledCount: 15,
    status: 'published',
    color: 'violet',
  },
  {
    id: 'crs_testing',
    title: 'Testing Strategies',
    description: 'Learn unit testing, integration testing, and test-driven development workflows.',
    category: 'Quality',
    instructorId: 'usr_instructor_01',
    instructorName: 'Ahmed Khalil',
    duration: '3 weeks',
    totalLessons: 6,
    totalModules: 2,
    enrolledCount: 12,
    status: 'draft',
    color: 'rose',
  },
];

export const demoModules = {
  crs_react: [
    {
      id: 'mod_r1',
      courseId: 'crs_react',
      title: 'Getting Started with React',
      order: 1,
      lessons: [
        { id: 'les_r1_1', title: 'Introduction to React', type: 'video', duration: '15 min', status: 'completed', order: 1 },
        { id: 'les_r1_2', title: 'Setting Up Your Environment', type: 'text', duration: '10 min', status: 'completed', order: 2 },
        { id: 'les_r1_3', title: 'Your First Component', type: 'video', duration: '20 min', status: 'completed', order: 3 },
      ],
    },
    {
      id: 'mod_r2',
      courseId: 'crs_react',
      title: 'Components & Props',
      order: 2,
      lessons: [
        { id: 'les_r2_1', title: 'Understanding Components', type: 'video', duration: '18 min', status: 'in_progress', order: 1 },
        { id: 'les_r2_2', title: 'Props and Data Flow', type: 'text', duration: '12 min', status: 'locked', order: 2 },
        { id: 'les_r2_3', title: 'Component Composition', type: 'video', duration: '22 min', status: 'locked', order: 3 },
      ],
    },
    {
      id: 'mod_r3',
      courseId: 'crs_react',
      title: 'State & Hooks',
      order: 3,
      lessons: [
        { id: 'les_r3_1', title: 'useState Hook', type: 'video', duration: '20 min', status: 'locked', order: 1 },
        { id: 'les_r3_2', title: 'useEffect Hook', type: 'video', duration: '25 min', status: 'locked', order: 2 },
        { id: 'les_r3_3', title: 'Custom Hooks', type: 'text', duration: '15 min', status: 'locked', order: 3 },
      ],
    },
    {
      id: 'mod_r4',
      courseId: 'crs_react',
      title: 'Routing & Navigation',
      order: 4,
      lessons: [
        { id: 'les_r4_1', title: 'React Router Setup', type: 'video', duration: '18 min', status: 'locked', order: 1 },
        { id: 'les_r4_2', title: 'Dynamic Routes', type: 'text', duration: '12 min', status: 'locked', order: 2 },
        { id: 'les_r4_3', title: 'Final Project', type: 'project', duration: '45 min', status: 'locked', order: 3 },
      ],
    },
  ],
};

/**
 * Student enrollment / progress – keyed by `studentId_courseId`
 * MOCK DATA: Replace with GET /enrollments or GET /students/:id/courses
 */
export const demoEnrollments = [
  { studentId: 'usr_student_01', courseId: 'crs_react', progress: 33, completedLessons: 4, status: 'in_progress', enrolledAt: '2026-09-15' },
  { studentId: 'usr_student_01', courseId: 'crs_go', progress: 20, completedLessons: 3, status: 'in_progress', enrolledAt: '2026-09-15' },
  { studentId: 'usr_student_01', courseId: 'crs_db', progress: 60, completedLessons: 6, status: 'in_progress', enrolledAt: '2026-09-20' },
  { studentId: 'usr_student_01', courseId: 'crs_api', progress: 0, completedLessons: 0, status: 'enrolled', enrolledAt: '2026-10-01' },
];

