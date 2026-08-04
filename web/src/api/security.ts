import request from './request'

export function getSecurityQuestions() {
  return request.get('/api/security/questions')
}

export function setSecurityQuestions(q1: string, a1: string, q2: string, a2: string, q3: string, a3: string) {
  return request.post('/api/security/questions', {
    question1: q1, answer1: a1,
    question2: q2, answer2: a2,
    question3: q3, answer3: a3,
  })
}

export function getQuestionsByUsername(username: string) {
  return request.post('/api/security/forgot/username', { username })
}

export function verifyAnswers(username: string, a1: string, a2: string, a3: string) {
  return request.post('/api/security/forgot/verify', {
    username,
    answer1: a1,
    answer2: a2,
    answer3: a3,
  })
}

export function verifyAndReset(username: string, a1: string, a2: string, a3: string, newPassword: string) {
  return request.post('/api/security/forgot/reset', {
    username,
    answer1: a1,
    answer2: a2,
    answer3: a3,
    new_password: newPassword,
  })
}
