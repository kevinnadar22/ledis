package commands

import "github.com/kevinnadar22/ledis/internal/datatypes"

type commandJob struct {
	sess  *Session
	cmd   datatypes.Command
	reply chan string
}

func (s *Server) startWorker() {
	go func() {
		for job := range s.jobs {
			job.reply <- job.sess.run(job.cmd)
		}
	}()
}

func (sess *Session) enqueue(cmd datatypes.Command) string {
	reply := make(chan string, 1)
	sess.srv.jobs <- commandJob{sess: sess, cmd: cmd, reply: reply}
	return <-reply
}
