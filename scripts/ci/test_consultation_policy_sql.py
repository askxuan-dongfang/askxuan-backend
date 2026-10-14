import re,sqlite3,json
from pathlib import Path
b=Path(__file__).resolve().parents[2]
db=sqlite3.connect(':memory:');db.create_function('NOW',0,lambda:'2026-10-13 12:00:00')
db.execute("ATTACH ':memory:' AS askxuan_booking")
db.execute('CREATE TABLE askxuan_booking.consultation_order(user_id,master_code,payment_status,status,consult_fee,valid_from,expires_at)')
db.execute('CREATE TABLE askxuan_booking.booking(booking_no,user_id,master_code,status,payment_status,payment_expire_time,slot_reserved,slot_code,total_fee)')
model=(b/'services/content/booking-service/internal/model/consultation.go').read_text()
query=re.search(r'"(SELECT COUNT\(1\) FROM consultation_order WHERE user_id=\?.*?)"',model)[1]
cashier=(b/'services/commerce/payment-service/internal/cashier/orders.go').read_text()
quote=next(x for x in re.findall(r'`(SELECT[^`]+)`',cashier) if 'FROM askxuan_booking.booking WHERE' in x)
base=['123','M001','success','active',39,'2026-10-12 12:00:00','2026-10-14 12:00:00']
cases=[('valid',{},True),('expired',{6:'2026-10-13 11:59:59'},False),('boundary',{6:'2026-10-13 12:00:00'},False),('future',{5:'2026-10-13 12:00:01'},False),('closed',{3:'closed'},False),('refunded',{2:'refunded'},False),('unpaid',{2:'pending'},False),('free',{4:0},False),('other-user',{0:'124'},False),('other-master',{1:'M002'},False),('null-expiry',{6:None},False)]
results=[]
for name,changes,expected in cases:
 row=base.copy()
 for k,v in changes.items():row[k]=v
 db.execute('DELETE FROM askxuan_booking.consultation_order');db.execute('INSERT INTO askxuan_booking.consultation_order VALUES(?,?,?,?,?,?,?)',row)
 actual=db.execute(query,['123','M001']).fetchone()[0]>0
 assert actual==expected,(name,'eligibility',actual)
 for reserved,slot in [(1,'AM'),(0,'')]:
  db.execute('DELETE FROM askxuan_booking.booking');db.execute('INSERT INTO askxuan_booking.booking VALUES(?,?,?,?,?,?,?,?,?)',['b','123','M001','pending_payment','pending','2026-10-13 13:00:00',reserved,slot,239])
  actual=bool(db.execute(quote,['b']).fetchone()[2]);assert actual==expected,(name,'cashier',reserved,actual)
 results.append({'case':name,'eligible':expected,'cashierBothTracks':expected})
db.execute('DELETE FROM askxuan_booking.consultation_order');db.execute("UPDATE askxuan_booking.booking SET master_code='',slot_reserved=1,slot_code='AM'")
assert bool(db.execute(quote,['b']).fetchone()[2])
print(json.dumps({'sourceQueries':'actual model eligibility + cashier payable SQL','engine':'SQLite compatible SQL predicate verification, not MySQL integration','cases':results,'wholeTempleWithoutConsultation':True},ensure_ascii=False,indent=2))
