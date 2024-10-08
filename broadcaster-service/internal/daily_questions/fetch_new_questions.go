package daily_questions

import (
	"fmt"
	"math"
	"time"

	"github.com/lib/pq"
	"github.com/renanmedina/dcp-broadcaster/internal/event_store"
	"github.com/renanmedina/dcp-broadcaster/utils"
)

type FetchNewQuestions struct {
	service             QuestionsService
	questionsRepository QuestionsRepository
	logger              *utils.ApplicationLogger
	publisher           *event_store.EventPublisher
}

func (uc *FetchNewQuestions) Execute() {
	latestQuestion := uc.questionsRepository.GetLatest()
	var fetchQuantity uint32 = 3

	if latestQuestion != nil {
		diff := time.Since(latestQuestion.ReceivedAt)
		fetchQuantity = uint32(math.Ceil(diff.Hours() / 24))
	}

	questions, err := uc.service.GetNewQuestions(fetchQuantity)

	if err != nil {
		uc.logger.Error(err.Error())
	}

	uc.processQuestions(questions)
}

func (uc *FetchNewQuestions) processQuestions(questions []Question) {
	for _, question := range questions {
		_, err := uc.questionsRepository.Save(question)

		if err != nil {
			if err, ok := err.(*pq.Error); ok {
				// ignore duplicate inserts of original_id, doing this here do make usage of db constraint and not need to manually load to check if exists
				if err.Code.Name() == UNIQUE_CONSTRAINT_ERROR {
					// uc.logger.Info("Ignored duplicated message received from questions service", "question", question.ToLogMap())
					continue
				}
			}

			errMsg := fmt.Sprintf("Failed processing message received from questions service: %s", err.Error())
			uc.logger.Error(errMsg, "question", question.ToLogMap())
			continue
		}

		uc.logger.Info("Processed message received from questions service", "question", question.ToLogMap())
		uc.publisher.Publish(newQuestionCreated(question))
	}
}

func NewFetchNewQuestions() (*FetchNewQuestions, error) {
	service := NewQuestionsService()

	return &FetchNewQuestions{
		service,
		NewQuestionsRepository(),
		utils.GetApplicationLogger(),
		newEventPublisher(),
	}, nil
}
