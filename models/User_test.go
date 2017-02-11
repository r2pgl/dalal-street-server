package models

import (
	"reflect"
	"sync"
	"testing"

	"gopkg.in/jarcoal/httpmock.v1"

	"github.com/thakkarparth007/dalal-street-server/utils/test"
)

func Test_Login(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", "https://api.pragyan.org/event/login", httpmock.NewStringResponder(200, `{"status_code":200,"message": { "user_id": "2", "user_fullname": "TestName" }}`))

	u, err := Login("test@testmail.com", "password")
	if err != nil {
		t.Fatalf("Login returned an error: %s", err)
	}

	defer func() {
		db, err := DbOpen()
		if err != nil {
			t.Fatal("Failed opening DB for cleaning up test user")
		}
		defer db.Close()

		db.Delete(u)
	}()

	exU := &User{
		Id:        2,
		Email:     "test@testmail.com",
		Name:      "TestName",
		Cash:      STARTING_CASH,
		Total:     STARTING_CASH,
		CreatedAt: u.CreatedAt,
	}
	if reflect.DeepEqual(u, exU) != true {
		t.Fatalf("Expected Login to return %+v, instead, got %+v", exU, u)
	}

	_, err = Login("test@testmail.com", "TestName")
	if err != nil {
		t.Fatalf("Login failed: '%s'", err)
	}

	//allErrors, ok = migrate.DownSync(connStr, "../migrations")
}

func TestUserToProto(t *testing.T) {
	o := &User{
		Id:        2,
		Email:     "test@testmail.com",
		Name:      "test user",
		Cash:      10000,
		Total:     -200,
		CreatedAt: "2017-06-08T00:00:00",
	}

	o_proto := o.ToProto()

	if !testutils.AssertEqual(t, o, o_proto) {
		t.Fatal("Converted values not equal!")
	}

}

func Test_PlaceAskOrder(t *testing.T) {
	var makeTrans = func(userId uint32, stockId uint32, transType TransactionType, stockQty int32, price uint32, total int32) *Transaction {
		return &Transaction{
			UserId:        userId,
			StockId:       stockId,
			Type:          transType,
			StockQuantity: stockQty,
			Price:         price,
			Total:         total,
		}
	}

	var makeAsk = func(userId uint32, stockId uint32, ot OrderType, stockQty uint32, price uint32) *Ask {
		return &Ask{
			UserId:        userId,
			StockId:       stockId,
			OrderType:     ot,
			StockQuantity: stockQty,
			Price:         price,
		}
	}

	var user = &User{Id: 2}
	var stock = &Stock{Id: 1}

	transactions := []*Transaction{
		makeTrans(2, 1, FromExchangeTransaction, 10, 200, 2000),
		makeTrans(2, 1, FromExchangeTransaction, -10, 200, 2000),
		makeTrans(2, 1, FromExchangeTransaction, -10, 200, 2000),
	}

	testcases := []struct {
		ask  *Ask
		pass bool
	}{
		{makeAsk(2, 1, Limit, 5, 200), true},
		{makeAsk(2, 1, Limit, 2, 200), true},
		{makeAsk(2, 1, Limit, 3, 200), true},
		{makeAsk(2, 1, Limit, 11, 200), false},
	}

	db, err := DbOpen()
	if err != nil {
		t.Fatal("Failed opening DB to insert dummy data")
	}
	defer func() {
		for _, tr := range transactions {
			db.Delete(tr)
		}
		for _, tc := range testcases {
			db.Delete(tc.ask)
		}
		db.Delete(stock)
		db.Delete(user)
		db.Close()

		delete(userLocks.m, 2)
	}()

	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(stock).Error; err != nil {
		t.Fatal(err)
	}

	for _, tr := range transactions {
		if err := db.Create(tr).Error; err != nil {
			t.Fatal(err)
		}
	}

	wg := sync.WaitGroup{}
	fm := sync.Mutex{}

	for _, tc := range testcases {
		if tc.pass != true {
			continue
		}
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := PlaceAskOrder(2, tc.ask)

			if err != nil {
				fm.Lock()
				defer fm.Unlock()
				t.Fatalf("Did not expect error. Got %+v", err)
			}

			a := &Ask{}
			db.First(a, tc.ask.Id)
			fm.Lock()
			defer fm.Unlock()
			if !testutils.AssertEqual(t, a, tc.ask) {
				t.Fatalf("Got %+v; Want %+v;", a, tc.ask)
			}
		}()
	}

	wg.Wait()

	id, err := PlaceAskOrder(2, testcases[len(testcases)-1].ask)
	if err == nil {
		t.Fatalf("Did not expect success. Failing %+v %+v", id, err)
	}
}

func Test_PlaceBidOrder(t *testing.T) {
	var makeTrans = func(userId uint32, stockId uint32, transType TransactionType, stockQty int32, price uint32, total int32) *Transaction {
		return &Transaction{
			UserId:        userId,
			StockId:       stockId,
			Type:          transType,
			StockQuantity: stockQty,
			Price:         price,
			Total:         total,
		}
	}

	var makeBid = func(userId uint32, stockId uint32, ot OrderType, stockQty uint32, price uint32) *Bid {
		return &Bid{
			UserId:        userId,
			StockId:       stockId,
			OrderType:     ot,
			StockQuantity: stockQty,
			Price:         price,
		}
	}

	var user = &User{Id: 2, Cash: 2000}
	var stock = &Stock{Id: 1}

	transactions := []*Transaction{
		makeTrans(2, 1, FromExchangeTransaction, 10, 200, 2000),
		makeTrans(2, 1, FromExchangeTransaction, -10, 200, 2000),
		makeTrans(2, 1, FromExchangeTransaction, -10, 200, 2000),
	}

	testcases := []struct {
		bid  *Bid
		pass bool
	}{
		{makeBid(2, 1, Limit, 5, 200), true},
		{makeBid(2, 1, Limit, 2, 200), true},
		{makeBid(2, 1, Limit, 3, 200), true},
		{makeBid(2, 1, Limit, 11, 200), false},
	}

	db, err := DbOpen()
	if err != nil {
		t.Fatal("Failed opening DB to insert dummy data")
	}
	defer func() {
		for _, tr := range transactions {
			db.Delete(tr)
		}
		for _, tc := range testcases {
			db.Delete(tc.bid)
		}
		db.Delete(stock)
		db.Delete(user)
		db.Close()

		delete(userLocks.m, 2)
	}()

	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(stock).Error; err != nil {
		t.Fatal(err)
	}

	for _, tr := range transactions {
		if err := db.Create(tr).Error; err != nil {
			t.Fatal(err)
		}
	}

	wg := sync.WaitGroup{}
	fm := sync.Mutex{}

	for _, tc := range testcases {
		if tc.pass != true {
			continue
		}
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := PlaceBidOrder(2, tc.bid)
			if err != nil {
				fm.Lock()
				defer fm.Unlock()
				t.Fatalf("Did not expect error. Got %+v", err)
			}

			b := &Bid{}
			db.First(b, tc.bid.Id)
			fm.Lock()
			defer fm.Unlock()
			if !testutils.AssertEqual(t, b, tc.bid) {
				t.Fatalf("Got %+v; Want %+v;", b, tc.bid)
			}
		}()
	}

	wg.Wait()

	id, err := PlaceBidOrder(2, testcases[len(testcases)-1].bid)
	if err == nil {
		t.Fatalf("Did not expect success. Failing %+v %+v", id, err)
	}
}

func Test_CancelOrder(t *testing.T) {
	var makeAsk = func(userId uint32, askId uint32, stockId uint32, ot OrderType, stockQty uint32, price uint32) *Ask {
		return &Ask{
			Id:            askId,
			UserId:        userId,
			StockId:       stockId,
			OrderType:     ot,
			StockQuantity: stockQty,
			Price:         price,
		}
	}

	var makeBid = func(userId uint32, bidId uint32, stockId uint32, ot OrderType, stockQty uint32, price uint32) *Bid {
		return &Bid{
			Id:            bidId,
			UserId:        userId,
			StockId:       stockId,
			OrderType:     ot,
			StockQuantity: stockQty,
			Price:         price,
		}
	}

	var user = &User{Id: 2}
	var stock = &Stock{Id: 1}

	var bids = []*Bid{
		makeBid(2, 150, 1, Limit, 5, 200),
		makeBid(2, 160, 1, Limit, 2, 200),
	}
	var asks = []*Ask{
		makeAsk(2, 150, 1, Limit, 5, 200),
		makeAsk(2, 160, 1, Limit, 2, 200),
	}

	testcases := []struct {
		userId  uint32
		orderId uint32
		isAsk   bool
		pass    bool
	}{
		{2, 150, false, true},
		{2, 160, false, true},
		{3, 150, false, false},
		{2, 250, false, false},
		{2, 150, true, true},
		{2, 160, true, true},
		{1, 150, false, false},
		{2, 260, false, false},
	}

	db, err := DbOpen()
	if err != nil {
		t.Fatal("Failed opening DB to insert dummy data")
	}
	defer func() {
		for _, a := range asks {
			db.Delete(a)
		}
		for _, b := range bids {
			db.Delete(b)
		}
		db.Delete(stock)
		db.Delete(user)
		db.Close()

		delete(userLocks.m, 2)
	}()

	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(stock).Error; err != nil {
		t.Fatal(err)
	}

	for _, a := range asks {
		if err := db.Create(a).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, b := range bids {
		if err := db.Create(b).Error; err != nil {
			t.Fatal(err)
		}
	}

	wg := sync.WaitGroup{}
	fm := sync.Mutex{}

	for _, tc := range testcases {
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := CancelOrder(tc.userId, tc.orderId, tc.isAsk)
			if tc.pass == true && err != nil {
				fm.Lock()
				defer fm.Unlock()
				t.Fatalf("Did not expect error. Got %+v", err)
			} else if tc.pass == false && err == nil {
				fm.Lock()
				defer fm.Unlock()
				t.Fatalf("Expected error. Didn't get it. Failing.")
			}
		}()
	}

	wg.Wait()

}

func Test_GetStocksOwned(t *testing.T) {
	var makeTrans = func(userId uint32, stockId uint32, transType TransactionType, stockQty int32, price uint32, total int32) *Transaction {
		return &Transaction{
			UserId:        userId,
			StockId:       stockId,
			Type:          transType,
			StockQuantity: stockQty,
			Price:         price,
			Total:         total,
		}
	}

	users := []*User{
		{Id: 2, Email: "a@b.com", Cash: 2000},
		{Id: 3, Email: "c@d.com", Cash: 1000},
		{Id: 4, Email: "e@f.com", Cash: 5000},
	}

	stocks := []*Stock{
		{Id: 1},
		{Id: 2},
		{Id: 3},
	}

	transactions := []*Transaction{
		makeTrans(2, 1, FromExchangeTransaction, 10, 1, 2000),
		makeTrans(2, 1, FromExchangeTransaction, 10, 2, 2000),
		makeTrans(2, 2, FromExchangeTransaction, -10, 1, 2000),

		makeTrans(3, 1, FromExchangeTransaction, 10, 1, 2000),
		makeTrans(3, 3, FromExchangeTransaction, -10, 2, 2000),

		makeTrans(4, 2, FromExchangeTransaction, -10, 2, 2000),
		makeTrans(4, 2, FromExchangeTransaction, 10, 1, 2000),
		makeTrans(4, 2, FromExchangeTransaction, -10, 1, 2000),
		makeTrans(4, 3, FromExchangeTransaction, 10, 1, 2000),
	}

	testcases := []struct {
		userId   uint32
		expected map[uint32]int32
	}{
		{userId: 2, expected: map[uint32]int32{1: 20, 2: -10}},
		{userId: 3, expected: map[uint32]int32{1: 10, 3: -10}},
		{userId: 4, expected: map[uint32]int32{2: -10, 3: 10}},
	}

	db, err := DbOpen()
	if err != nil {
		t.Fatal("Failed opening DB to insert dummy data")
	}
	defer func() {
		for _, tr := range transactions {
			if err := db.Delete(tr).Error; err != nil {
				t.Fatal(err)
			}
		}
		for _, stock := range stocks {
			if err := db.Delete(stock).Error; err != nil {
				t.Fatal(err)
			}
		}
		for _, user := range users {
			if err := db.Delete(user).Error; err != nil {
				t.Fatal(err)
			}
			delete(userLocks.m, user.Id)
		}

		db.Close()
	}()

	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, stock := range stocks {
		if err := db.Create(stock).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tr := range transactions {
		if err := db.Create(tr).Error; err != nil {
			t.Fatal(err)
		}
	}

	wg := sync.WaitGroup{}
	fm := sync.Mutex{}

	for _, tc := range testcases {
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			ret, err := GetStocksOwned(tc.userId)
			fm.Lock()
			defer fm.Unlock()

			if err != nil {
				t.Fatalf("Did not expect error. Got %+v", err)
			}
			if !testutils.AssertEqual(t, tc.expected, ret) {
				t.Fatalf("Got %+v; want %+v", ret, tc.expected)
			}
		}()
	}

	wg.Wait()
}
